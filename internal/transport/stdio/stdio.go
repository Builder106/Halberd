// Package stdio implements Halberd's stdio transport: a man-in-the-middle
// wrapper that sits between an MCP host (Claude Desktop, Cursor, Windsurf)
// and a stdio-speaking MCP server. The host fork-execs halberd-stdio in
// place of the real server; halberd-stdio fork-execs the real server as
// its own child and pipes JSON-RPC messages through the policy engine in
// both directions.
//
// MCP's stdio transport is plain line-delimited JSON-RPC (one message per
// newline), so this package uses ordinary exec.Cmd pipes rather than a PTY.
// PTY allocation would risk corrupting binary content in tool arguments via
// line-discipline translation.
package stdio

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/Builder106/halberd/internal/audit"
	"github.com/Builder106/halberd/internal/jsonrpc"
	"github.com/Builder106/halberd/internal/policy"
)

// MaxLineBytes caps a single JSON-RPC message at 4 MiB. Messages larger
// than this are dropped and an audit entry is written.
const MaxLineBytes = 4 << 20

// HostStreams is the JSON-RPC channel that halberd-stdio presents to its
// parent process — typically the MCP host.
type HostStreams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// ChildStreams is the JSON-RPC channel that halberd-stdio drives toward
// the upstream MCP server it has fork-exec'd.
type ChildStreams struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
	Stderr io.Reader
}

// Wrap runs the bidirectional JSON-RPC pipe between host and child. Every
// inbound message from the host is evaluated by engine; allowed messages
// are forwarded to the child, blocked requests receive a synthetic
// JSON-RPC error response, blocked notifications are dropped silently.
// Outbound messages from the child stream through to the host unchanged
// (response inspection lands in P4). stderr from the child is forwarded
// transparently to the host's stderr so server diagnostics still surface.
//
// Wrap returns when ctx is cancelled, when the host closes its input, or
// when the child closes its output. On cancellation it closes every
// closable pipe endpoint and waits for the I/O workers to stop before
// returning. The caller is responsible for reaping the child process and
// exposing its exit status.
func Wrap(ctx context.Context, engine *policy.Engine, bus *audit.Bus, host HostStreams, child ChildStreams) error {
	var outMu sync.Mutex
	writeHostLine := func(b []byte) error {
		outMu.Lock()
		defer outMu.Unlock()
		if _, err := host.Out.Write(b); err != nil {
			return err
		}
		_, err := host.Out.Write([]byte{'\n'})
		return err
	}

	var wg sync.WaitGroup
	wg.Add(3)

	// host stdin -> policy -> child stdin
	go func() {
		defer wg.Done()
		defer func() { _ = child.Stdin.Close() }()

		sc := bufio.NewScanner(host.In)
		sc.Buffer(make([]byte, 64<<10), MaxLineBytes)
		for sc.Scan() {
			line := sc.Bytes()
			decision := engine.EvaluateRequest(line)

			method, tool := peekMethodTool(line)
			bus.Record(audit.Event{
				Direction:  "request",
				Method:     method,
				Tool:       tool,
				Blocked:    decision.Blocked,
				Violations: decision.Violations,
			})

			if decision.Blocked {
				id, hasID := extractID(line)
				if !hasID {
					// Notification: spec forbids a response. Drop silently;
					// the audit entry above is the only record.
					continue
				}
				resp, _ := jsonrpc.PolicyViolation(id, summarize(decision), decision.Violations)
				if err := writeHostLine(resp); err != nil {
					slog.Error("write blocked response to host", "err", err)
					return
				}
				continue
			}

			// Allowed — forward to child. Re-append the newline that
			// bufio.Scanner stripped.
			if _, err := child.Stdin.Write(line); err != nil {
				slog.Error("write to child stdin", "err", err)
				return
			}
			if _, err := child.Stdin.Write([]byte{'\n'}); err != nil {
				slog.Error("write newline to child stdin", "err", err)
				return
			}
		}
		if err := sc.Err(); err != nil {
			slog.Error("scan host stdin", "err", err)
		}
	}()

	// child stdout -> response inspection -> host stdout
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(child.Stdout)
		sc.Buffer(make([]byte, 64<<10), MaxLineBytes)
		inspectResponses := engine.HasResponseFilters()
		for sc.Scan() {
			line := sc.Bytes()
			payload := line

			if inspectResponses {
				// EvaluateResponse needs its own copy because bufio.Scanner
				// reuses the underlying buffer on the next Scan call.
				owned := make([]byte, len(line))
				copy(owned, line)
				result := engine.EvaluateResponse(owned)
				payload = result.Payload
				if len(result.Detections) > 0 {
					bus.Record(audit.Event{
						Direction:  "response",
						Violations: result.Detections,
					})
				}
			}

			if err := writeHostLine(payload); err != nil {
				slog.Error("write to host stdout", "err", err)
				return
			}
		}
		if err := sc.Err(); err != nil {
			slog.Error("scan child stdout", "err", err)
		}
	}()

	// child stderr -> host stderr (transparent passthrough so server logs
	// reach whatever console the host shows the user)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(host.Err, child.Stderr); err != nil {
			slog.Debug("child stderr copy ended", "err", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		closeReader(host.In)
		_ = child.Stdin.Close()
		closeReader(child.Stdout)
		closeReader(child.Stderr)
		<-done
		return ctx.Err()
	}
}

func closeReader(r io.Reader) {
	closer, ok := r.(io.Closer)
	if !ok {
		return
	}
	_ = closer.Close()
}

func peekMethodTool(line []byte) (method, tool string) {
	var env struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(line, &env); err != nil {
		return "", ""
	}
	return env.Method, env.Params.Name
}

func extractID(line []byte) (json.RawMessage, bool) {
	var env struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, false
	}
	// Distinguish "id absent" (notification) from "id: null" (allowed by
	// the spec for requests). bufio gives us the literal bytes either way.
	if len(env.ID) == 0 {
		return nil, false
	}
	return env.ID, true
}

func summarize(d policy.Decision) string {
	if len(d.Violations) == 0 {
		return "halberd: request blocked by policy"
	}
	v := d.Violations[0]
	if v.Field != "" {
		return fmt.Sprintf("halberd: %s on %s", v.Rule, v.Field)
	}
	return fmt.Sprintf("halberd: %s on %s", v.Rule, v.Tool)
}

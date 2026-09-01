package stdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Builder106/halberd/internal/audit"
	"github.com/Builder106/halberd/internal/jsonrpc"
	"github.com/Builder106/halberd/internal/policy"
)

const testBundle = `
version: 1
server: test
tools:
  - name: query
    arguments:
      sql:
        type: string
        deny_patterns: ['(?i)\bdrop\s+table\b']
defaults: { unknown_tool: deny, unknown_method: log_and_pass }
`

// rig wires four io.Pipes together to simulate host and child without
// fork-execing anything. The fake "child" runs in a goroutine: it reads a
// line from its stdin, asserts on it, and writes a result to its stdout.
type rig struct {
	hostIn      *io.PipeWriter
	hostOut     *bufio.Reader
	hostOutPipe *io.PipeReader
	childStdin  *bufio.Reader
	childStdout *io.PipeWriter
	childStderr *io.PipeWriter
	auditBuf    *bytes.Buffer
	bus         *audit.Bus
	wrapDone    chan error
	cancel      context.CancelFunc
}

func newRig(t *testing.T) *rig {
	t.Helper()
	bundle, err := policy.ParseBundle([]byte(testBundle))
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	engine := policy.New(bundle)

	hostInR, hostInW := io.Pipe()
	hostOutR, hostOutW := io.Pipe()
	childInR, childInW := io.Pipe()
	childOutR, childOutW := io.Pipe()
	childErrR, childErrW := io.Pipe()

	auditBuf := &bytes.Buffer{}
	bus := audit.NewBus(auditBuf, 16)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Wrap(ctx, engine, bus, HostStreams{
			In:  hostInR,
			Out: hostOutW,
			Err: io.Discard,
		}, ChildStreams{
			Stdin:  childInW,
			Stdout: childOutR,
			Stderr: childErrR,
		})
		_ = hostOutW.Close()
	}()

	return &rig{
		hostIn:      hostInW,
		hostOut:     bufio.NewReader(hostOutR),
		hostOutPipe: hostOutR,
		childStdin:  bufio.NewReader(childInR),
		childStdout: childOutW,
		childStderr: childErrW,
		auditBuf:    auditBuf,
		bus:         bus,
		wrapDone:    done,
		cancel:      cancel,
	}
}

func (r *rig) close(t *testing.T) {
	t.Helper()
	_ = r.hostIn.Close()
	_ = r.childStdout.Close()
	_ = r.childStderr.Close()
	r.cancel()
	select {
	case <-r.wrapDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Wrap did not return within 2s of close")
	}
	_ = r.bus.Stop(context.Background())
}

// readChildLine reads one JSON-RPC line that the wrapper forwarded to the
// child, with a timeout so a stuck test fails loudly instead of hanging.
func readChildLine(t *testing.T, r *rig) string {
	t.Helper()
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		line, err := r.childStdin.ReadString('\n')
		ch <- res{line, err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("read child stdin: %v", got.err)
		}
		return strings.TrimRight(got.line, "\n")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for line on child stdin")
		return ""
	}
}

func readHostLine(t *testing.T, r *rig) string {
	t.Helper()
	type res struct {
		line string
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		line, err := r.hostOut.ReadString('\n')
		ch <- res{line, err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("read host stdout: %v", got.err)
		}
		return strings.TrimRight(got.line, "\n")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for line on host stdout")
		return ""
	}
}

func TestWrap_ForwardsAllowed(t *testing.T) {
	r := newRig(t)
	defer r.close(t)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`
	if _, err := r.hostIn.Write([]byte(req + "\n")); err != nil {
		t.Fatalf("write host stdin: %v", err)
	}

	got := readChildLine(t, r)
	if got != req {
		t.Fatalf("child saw %q, want %q", got, req)
	}

	// Fake child responds. The wrapper should forward unchanged.
	resp := `{"jsonrpc":"2.0","id":1,"result":{"rows":[]}}`
	if _, err := r.childStdout.Write([]byte(resp + "\n")); err != nil {
		t.Fatalf("write child stdout: %v", err)
	}
	if got := readHostLine(t, r); got != resp {
		t.Fatalf("host saw %q, want %q", got, resp)
	}
}

func TestWrap_BlocksRequestWithSyntheticError(t *testing.T) {
	r := newRig(t)
	defer r.close(t)

	req := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`
	if _, err := r.hostIn.Write([]byte(req + "\n")); err != nil {
		t.Fatalf("write host stdin: %v", err)
	}

	line := readHostLine(t, r)
	var resp jsonrpc.Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode response: %v\nline: %s", err, line)
	}
	if resp.Error == nil || resp.Error.Code != jsonrpc.CodePolicyViolation {
		t.Fatalf("expected policy violation, got %+v", resp)
	}
	if string(resp.ID) != "42" {
		t.Errorf("response id = %s, want 42", string(resp.ID))
	}

	// And nothing should have reached the child.
	type res struct{ line string }
	ch := make(chan res, 1)
	go func() {
		line, _ := r.childStdin.ReadString('\n')
		ch <- res{line}
	}()
	select {
	case got := <-ch:
		t.Fatalf("blocked request leaked to child: %q", got.line)
	case <-time.After(150 * time.Millisecond):
		// good — child saw nothing
	}
}

func TestWrap_DropsBlockedNotificationSilently(t *testing.T) {
	r := newRig(t)
	defer r.close(t)

	// Notification — no `id` field. The spec forbids a response.
	notif := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`
	if _, err := r.hostIn.Write([]byte(notif + "\n")); err != nil {
		t.Fatalf("write host stdin: %v", err)
	}

	// Spawn exactly one reader. If a line arrives, the test fails. If the
	// pipe closes (which it will when close() tears down host stdin and
	// Wrap closes the host stdout writer), the read returns an error and
	// the goroutine exits cleanly — no race on the bufio.Reader.
	hostSaw := make(chan string, 1)
	go func() {
		if line, err := r.hostOut.ReadString('\n'); err == nil && line != "" {
			hostSaw <- line
		}
	}()

	select {
	case line := <-hostSaw:
		t.Fatalf("blocked notification produced a host response: %q", line)
	case <-time.After(150 * time.Millisecond):
		// good — no response, as the spec requires
	}

	// Audit log should still record the block. Force a drain by stopping
	// the bus and reading what landed.
	_ = r.bus.Stop(context.Background())
	if !strings.Contains(r.auditBuf.String(), `"blocked":true`) {
		t.Errorf("audit log missing blocked entry: %q", r.auditBuf.String())
	}
}

const bundleWithResponseFilters = `
version: 1
server: test
tools:
  - name: query
    arguments: {}
defaults: { unknown_tool: deny, unknown_method: log_and_pass }
response_filters:
  global:
    secret_scanners: [aws_access_key]
`

func newRigWithBundle(t *testing.T, src string) *rig {
	t.Helper()
	bundle, err := policy.ParseBundle([]byte(src))
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	engine := policy.New(bundle)

	hostInR, hostInW := io.Pipe()
	hostOutR, hostOutW := io.Pipe()
	childInR, childInW := io.Pipe()
	childOutR, childOutW := io.Pipe()
	childErrR, childErrW := io.Pipe()

	auditBuf := &bytes.Buffer{}
	bus := audit.NewBus(auditBuf, 16)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Wrap(ctx, engine, bus, HostStreams{
			In: hostInR, Out: hostOutW, Err: io.Discard,
		}, ChildStreams{
			Stdin: childInW, Stdout: childOutR, Stderr: childErrR,
		})
		_ = hostOutW.Close()
	}()

	return &rig{
		hostIn:      hostInW,
		hostOut:     bufio.NewReader(hostOutR),
		hostOutPipe: hostOutR,
		childStdin:  bufio.NewReader(childInR),
		childStdout: childOutW,
		childStderr: childErrW,
		auditBuf:    auditBuf,
		bus:         bus,
		wrapDone:    done,
		cancel:      cancel,
	}
}

func TestWrap_SanitizesResponse(t *testing.T) {
	r := newRigWithBundle(t, bundleWithResponseFilters)
	defer r.close(t)

	// Feed an allowed request so the wrapper is in steady state.
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{}}}`
	if _, err := r.hostIn.Write([]byte(req + "\n")); err != nil {
		t.Fatalf("write host stdin: %v", err)
	}
	_ = readChildLine(t, r)

	// Fake child response carrying an AWS key.
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"db user AKIAIOSFODNN7EXAMPLE"}]}}`
	if _, err := r.childStdout.Write([]byte(resp + "\n")); err != nil {
		t.Fatalf("write child stdout: %v", err)
	}

	line := readHostLine(t, r)
	if strings.Contains(line, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked through stdio response inspector: %q", line)
	}
	if !strings.Contains(line, "[REDACTED]") {
		t.Errorf("redaction placeholder missing: %q", line)
	}
}

func TestWrap_ChildStderrPassesThrough(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)

	hostInR, hostInW := io.Pipe()
	hostOutR, hostOutW := io.Pipe()
	childInR, childInW := io.Pipe()
	childOutR, childOutW := io.Pipe()
	childErrR, childErrW := io.Pipe()

	bus := audit.NewBus(&bytes.Buffer{}, 16)
	hostErr := &syncBuf{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Wrap(ctx, engine, bus, HostStreams{In: hostInR, Out: hostOutW, Err: hostErr},
			ChildStreams{Stdin: childInW, Stdout: childOutR, Stderr: childErrR})
	}()

	if _, err := childErrW.Write([]byte("connecting to postgres...\n")); err != nil {
		t.Fatalf("write child stderr: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(hostErr.String(), "connecting to postgres") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(hostErr.String(), "connecting to postgres") {
		t.Fatalf("stderr did not reach host: %q", hostErr.String())
	}

	_ = hostInW.Close()
	_ = childErrW.Close()
	_ = childOutW.Close()
	_ = hostOutR.Close()
	_ = childInR.Close()
	cancel()
	<-done
	_ = bus.Stop(context.Background())
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestStdio_Helpers(t *testing.T) {
	// peekMethodTool
	m, tool := peekMethodTool([]byte("invalid json"))
	if m != "" || tool != "" {
		t.Errorf("expected empty on invalid json, got %q, %q", m, tool)
	}

	// extractID
	if _, ok := extractID([]byte("invalid json")); ok {
		t.Error("expected ok=false on invalid json")
	}
	if _, ok := extractID([]byte("{}")); ok {
		t.Error("expected ok=false when ID is absent")
	}
	if id, ok := extractID([]byte(`{"id":123}`)); !ok || string(id) != "123" {
		t.Errorf("expected 123, got %s, %v", string(id), ok)
	}

	// summarize
	if s := summarize(policy.Decision{}); s != "halberd: request blocked by policy" {
		t.Errorf("unexpected summarize empty: %s", s)
	}
	if s := summarize(policy.Decision{Violations: []policy.Violation{{Rule: "unknown_tool", Tool: "bash"}}}); s != "halberd: unknown_tool on bash" {
		t.Errorf("unexpected summarize tool: %s", s)
	}
	if s := summarize(policy.Decision{Violations: []policy.Violation{{Rule: "deny_pattern", Field: "sql"}}}); s != "halberd: deny_pattern on sql" {
		t.Errorf("unexpected summarize field: %s", s)
	}
}

type failingReader struct {
	err error
}

func (f *failingReader) Read(_ []byte) (int, error) {
	return 0, f.err
}

type failAfterNWriter struct {
	n   int
	err error
}

func (f *failAfterNWriter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, f.err
	}
	f.n--
	return len(p), nil
}

func (f *failAfterNWriter) Close() error {
	return nil
}

type blockingReadCloser struct {
	readStarted chan struct{}
	readDone    chan struct{}
	closed      chan struct{}
	startOnce   sync.Once
	closeOnce   sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		readStarted: make(chan struct{}),
		readDone:    make(chan struct{}),
		closed:      make(chan struct{}),
	}
}

func (r *blockingReadCloser) Read(_ []byte) (int, error) {
	r.startOnce.Do(func() { close(r.readStarted) })
	<-r.closed
	close(r.readDone)
	return 0, io.ErrClosedPipe
}

func (r *blockingReadCloser) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

func TestWrap_ContextCancellation(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	hostIn := newBlockingReadCloser()
	childStdout := newBlockingReadCloser()
	childStderr := newBlockingReadCloser()
	t.Cleanup(func() {
		_ = hostIn.Close()
		_ = childStdout.Close()
		_ = childStderr.Close()
	})

	wrapDone := make(chan error, 1)
	go func() {
		wrapDone <- Wrap(ctx, engine, bus,
			HostStreams{In: hostIn, Out: io.Discard, Err: io.Discard},
			ChildStreams{
				Stdin:  &failAfterNWriter{n: 1},
				Stdout: childStdout,
				Stderr: childStderr,
			})
	}()

	for _, started := range []<-chan struct{}{hostIn.readStarted, childStdout.readStarted, childStderr.readStarted} {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("stdio worker did not block on its input")
		}
	}

	cancel()
	select {
	case err := <-wrapDone:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wrap did not return after cancellation")
	}

	for _, stopped := range []<-chan struct{}{hostIn.readDone, childStdout.readDone, childStderr.readDone} {
		select {
		case <-stopped:
		default:
			t.Fatal("Wrap returned before a blocked stdio worker stopped")
		}
	}
}

func TestWrap_WriteHostErrorPaths(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Error on writing blocked response to host
	hostInR, hostInW := io.Pipe()
	childInR, childInW := io.Pipe()
	childOutR, childOutW := io.Pipe()
	go func() {
		_ = Wrap(ctx, engine, bus, HostStreams{
			In:  hostInR,
			Out: &failAfterNWriter{n: 0, err: io.ErrClosedPipe},
			Err: io.Discard,
		}, ChildStreams{
			Stdin:  childInW,
			Stdout: childOutR,
			Stderr: bytes.NewReader(nil),
		})
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`
	_, _ = hostInW.Write([]byte(req + "\n"))
	time.Sleep(20 * time.Millisecond)
	_ = hostInW.Close()
	_ = childInR.Close()
	_ = childOutW.Close()
}

func TestWrap_ChildStdinWriteError(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostInR, hostInW := io.Pipe()
	childOutR, childOutW := io.Pipe()
	go func() {
		_ = Wrap(ctx, engine, bus, HostStreams{
			In:  hostInR,
			Out: io.Discard,
			Err: io.Discard,
		}, ChildStreams{
			Stdin:  &failAfterNWriter{n: 0, err: io.ErrClosedPipe},
			Stdout: childOutR,
			Stderr: bytes.NewReader(nil),
		})
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`
	_, _ = hostInW.Write([]byte(req + "\n"))
	time.Sleep(20 * time.Millisecond)
	_ = hostInW.Close()
	_ = childOutW.Close()

	// Test second write failure (newline write failure)
	hostInR2, hostInW2 := io.Pipe()
	childOutR2, _ := io.Pipe()
	go func() {
		_ = Wrap(ctx, engine, bus, HostStreams{
			In:  hostInR2,
			Out: io.Discard,
			Err: io.Discard,
		}, ChildStreams{
			Stdin:  &failAfterNWriter{n: 1, err: io.ErrClosedPipe},
			Stdout: childOutR2,
			Stderr: bytes.NewReader(nil),
		})
	}()

	_, _ = hostInW2.Write([]byte(req + "\n"))
	time.Sleep(20 * time.Millisecond)
	_ = hostInW2.Close()
	_ = childOutR2.Close()
}

func TestWrap_ScanErrorsAndChildErrors(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	childInR, childInW := io.Pipe()
	defer func() { _ = childInR.Close(); _ = childInW.Close() }()

	// Scanner error on host In, error on child Stdout, and error on child Stderr
	errHostIn := &failingReader{err: io.ErrUnexpectedEOF}
	errChildOut := &failingReader{err: io.ErrUnexpectedEOF}
	errChildErr := &failingReader{err: io.ErrUnexpectedEOF}

	_ = Wrap(ctx, engine, bus, HostStreams{
		In:  errHostIn,
		Out: io.Discard,
		Err: io.Discard,
	}, ChildStreams{
		Stdin:  childInW,
		Stdout: errChildOut,
		Stderr: errChildErr,
	})
}

func TestWrap_ChildStdoutWriteHostError(t *testing.T) {
	bundle, _ := policy.ParseBundle([]byte(testBundle))
	engine := policy.New(bundle)
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostInR, hostInW := io.Pipe()
	defer func() { _ = hostInR.Close(); _ = hostInW.Close() }()
	childInR, childInW := io.Pipe()
	defer func() { _ = childInR.Close(); _ = childInW.Close() }()
	childOutR, childOutW := io.Pipe()

	go func() {
		_ = Wrap(ctx, engine, bus, HostStreams{
			In:  hostInR,
			Out: &failAfterNWriter{n: 0, err: io.ErrClosedPipe},
			Err: io.Discard,
		}, ChildStreams{
			Stdin:  childInW,
			Stdout: childOutR,
			Stderr: bytes.NewReader(nil),
		})
	}()

	_, _ = childOutW.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}` + "\n"))
	time.Sleep(20 * time.Millisecond)
	_ = childOutW.Close()
}

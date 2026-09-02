// Package http wraps net/http/httputil.ReverseProxy with Halberd's policy
// engine. The handler reads the JSON-RPC request body, evaluates it against
// the engine, records the decision to the audit bus, and either forwards the
// request to the upstream MCP server or returns a synthetic policy-violation
// error response.
package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Builder106/halberd/internal/audit"
	"github.com/Builder106/halberd/internal/jsonrpc"
	"github.com/Builder106/halberd/internal/policy"
)

const (
	maxRequestBytes  = 4 << 20 // 4 MiB ceiling on JSON-RPC envelope
	maxResponseBytes = 8 << 20 // Maximum buffered response body (8 MiB)
)

// NewHandler returns an http.Handler that reverse-proxies JSON-RPC requests
// to target, gating each POST on engine.EvaluateRequest. Decisions are
// pushed to bus regardless of outcome. The underlying httputil.ReverseProxy
// flushes on every write (FlushInterval=-1) so SSE streamed responses from
// the upstream MCP server reach the agent without buffering.
func NewHandler(target *url.URL, engine *policy.Engine, bus *audit.Bus) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// FlushInterval -1 causes the proxy to flush on every Write, which is
	// what SSE needs to stream tool/list_changed and tools/call result chunks
	// to the agent without buffering.
	proxy.FlushInterval = -1

	origDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		r.Host = target.Host
	}

	if engine.HasResponseFilters() {
		proxy.ModifyResponse = func(resp *http.Response) error {
			// SSE responses stream multiple JSON-RPC messages and can't be
			// safely buffered as a whole. v0.1 of response inspection skips
			// them; the v0.2 roadmap adds per-event inspection.
			if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
				return nil
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
			if err != nil {
				return err
			}
			_ = resp.Body.Close()
			if len(body) > maxResponseBytes {
				return errors.New("upstream response body exceeds size limit")
			}

			result := engine.EvaluateResponse(body)
			if len(result.Detections) > 0 {
				bus.Record(audit.Event{
					Direction:  "response",
					Violations: result.Detections,
				})
			}

			payload := result.Payload
			resp.Body = io.NopCloser(bytes.NewReader(payload))
			resp.ContentLength = int64(len(payload))
			resp.Header.Set("Content-Length", strconv.Itoa(len(payload)))
			return nil
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			proxy.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		_ = r.Body.Close()

		decision := engine.EvaluateRequest(body)

		method, tool := peek(body)
		bus.Record(audit.Event{
			Direction:  "request",
			Method:     method,
			Tool:       tool,
			Blocked:    decision.Blocked,
			Violations: decision.Violations,
			RemoteAddr: r.RemoteAddr,
		})

		if decision.Blocked {
			writePolicyViolation(w, body, decision)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		proxy.ServeHTTP(w, r)
	})
}

func writePolicyViolation(w http.ResponseWriter, requestBody []byte, d policy.Decision) {
	id := extractID(requestBody)

	summary := "halberd: request blocked by policy"
	if len(d.Violations) > 0 {
		summary = "halberd: " + d.Violations[0].Rule + " on " + d.Violations[0].Field
	}

	resp, _ := jsonrpc.PolicyViolation(id, summary, d.Violations)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors ride a 200 with error in body
	_, _ = w.Write(resp)
}

func peek(body []byte) (method, tool string) {
	var env struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", ""
	}
	return env.Method, env.Params.Name
}

func extractID(body []byte) json.RawMessage {
	var env struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	return env.ID
}

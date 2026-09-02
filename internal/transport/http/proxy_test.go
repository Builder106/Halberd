package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

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

func newTestProxy(t *testing.T, upstream http.Handler) (handler http.Handler, audited *bytes.Buffer, cleanup func()) {
	t.Helper()
	srv := httptest.NewServer(upstream)
	u, _ := url.Parse(srv.URL)

	bundle, err := policy.ParseBundle([]byte(testBundle))
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}

	auditBuf := &bytes.Buffer{}
	bus := audit.NewBus(auditBuf, 16)
	engine := policy.New(bundle)
	return NewHandler(u, engine, bus), auditBuf, func() {
		srv.Close()
		_ = bus // bus drains async; tests that inspect audit log call time.Sleep before reading
	}
}

func TestProxy_ForwardsAllowedRequest(t *testing.T) {
	upstreamHit := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "SELECT") {
			t.Errorf("upstream did not see SELECT, got %s", body)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	})

	h, _, cleanup := newTestProxy(t, upstream)
	defer cleanup()

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !upstreamHit {
		t.Fatal("upstream not reached for allowed request")
	}
}

func TestProxy_BlocksDropTable(t *testing.T) {
	upstreamHit := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	})

	h, _, cleanup := newTestProxy(t, upstream)
	defer cleanup()

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE students"}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)

	if upstreamHit {
		t.Fatal("upstream reached despite policy block")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JSON-RPC error in body)", w.Code)
	}

	var resp jsonrpc.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != jsonrpc.CodePolicyViolation {
		t.Fatalf("expected policy violation error, got %+v", resp)
	}
	if string(resp.ID) != "7" {
		t.Errorf("response id = %s, want 7", string(resp.ID))
	}
}

const responseFilterBundle = `
version: 1
server: test
tools:
  - name: query
    arguments: {}
defaults: { unknown_tool: deny, unknown_method: log_and_pass }
response_filters:
  global:
    strip_ansi_escapes: true
    secret_scanners: [aws_access_key]
`

func TestProxy_SanitizesResponse(t *testing.T) {
	// Upstream sends a response with an embedded AWS key. Halberd must
	// redact it before the client sees the body.
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"db user AKIAIOSFODNN7EXAMPLE"}]}}`))
	})
	srv := httptest.NewServer(upstream)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	bundle, err := policy.ParseBundle([]byte(responseFilterBundle))
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()
	h := NewHandler(u, policy.New(bundle), bus)

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)

	if strings.Contains(w.Body.String(), "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked through response inspector: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "[REDACTED]") {
		t.Errorf("redaction placeholder missing: %s", w.Body.String())
	}
	if cl := w.Header().Get("Content-Length"); cl != strconv.Itoa(w.Body.Len()) {
		t.Errorf("Content-Length = %q, want %d", cl, w.Body.Len())
	}
}

func TestProxy_ResponseBodySizeLimit(t *testing.T) {
	tests := []struct {
		name       string
		response   []byte
		wantStatus int
	}{
		{
			name:       "exact limit",
			response:   responseBodyOfSize(t, maxResponseBytes),
			wantStatus: http.StatusOK,
		},
		{
			name:       "over limit",
			response:   responseBodyOfSize(t, maxResponseBytes+1),
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(tt.response)
			})
			srv := httptest.NewServer(upstream)
			defer srv.Close()

			u, _ := url.Parse(srv.URL)
			bundle, err := policy.ParseBundle([]byte(responseFilterBundle))
			if err != nil {
				t.Fatalf("bundle: %v", err)
			}
			bus := audit.NewBus(&bytes.Buffer{}, 16)
			defer func() { _ = bus.Stop(context.Background()) }()
			h := NewHandler(u, policy.New(bundle), bus)

			w := httptest.NewRecorder()
			body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{}}}`)
			r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			h.ServeHTTP(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK && !bytes.Equal(w.Body.Bytes(), tt.response) {
				t.Fatal("response at the size limit was not forwarded intact")
			}
		})
	}
}

func responseBodyOfSize(t *testing.T, size int) []byte {
	t.Helper()
	prefix := []byte(`{"jsonrpc":"2.0","id":1,"result":{"text":"`)
	suffix := []byte(`"}}`)
	paddingSize := size - len(prefix) - len(suffix)
	if paddingSize < 0 {
		t.Fatalf("response size %d is too small for the JSON-RPC envelope", size)
	}

	body := make([]byte, 0, size)
	body = append(body, prefix...)
	body = append(body, bytes.Repeat([]byte("A"), paddingSize)...)
	body = append(body, suffix...)
	return body
}

func TestProxy_NonPostPassesThrough(t *testing.T) {
	upstreamHit := false
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHit = true
		_, _ = w.Write([]byte("ok"))
	})

	h, _, cleanup := newTestProxy(t, upstream)
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(w, r)

	if !upstreamHit {
		t.Fatal("GET should pass through to upstream")
	}
}

func TestProxy_RequestBodyTooLarge(t *testing.T) {
	h, _, cleanup := newTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer cleanup()

	// 5MB body exceeds maxRequestBytes (4MB)
	largeBody := bytes.Repeat([]byte("A"), 5<<20)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(largeBody))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

func TestProxy_MalformedJSON(t *testing.T) {
	h, _, cleanup := newTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer cleanup()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{invalid-json")))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestProxy_SSEResponsePassthrough(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: test stream\n\n"))
	})
	srv := httptest.NewServer(upstream)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	bundle, err := policy.ParseBundle([]byte(responseFilterBundle))
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()
	h := NewHandler(u, policy.New(bundle), bus)

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)

	if !strings.Contains(w.Body.String(), "data: test stream") {
		t.Fatalf("expected SSE stream passthrough, got %s", w.Body.String())
	}
}

func TestProxy_BlockSummaryWithoutField(t *testing.T) {
	// Unknown tool blocked without field
	h, _, cleanup := newTestProxy(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer cleanup()

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"unknown_tool","arguments":{}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp jsonrpc.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "unknown_tool on ") {
		t.Errorf("unexpected error message: %+v", resp.Error)
	}
}

func TestProxy_ModifyResponse_ReadError(t *testing.T) {
	upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n"))
		_ = conn.Close()
	})
	srv := httptest.NewServer(upstream)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	bundle, err := policy.ParseBundle([]byte(responseFilterBundle))
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	bus := audit.NewBus(&bytes.Buffer{}, 16)
	defer func() { _ = bus.Stop(context.Background()) }()
	h := NewHandler(u, policy.New(bundle), bus)

	w := httptest.NewRecorder()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{}}}`)
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected Bad Gateway (502) on upstream read error, got %d", w.Code)
	}
}

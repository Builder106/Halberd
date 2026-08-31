package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

const bundleWithResponseFilters = `
version: 1
server: test
tools:
  - name: query
    arguments: {}
defaults: { unknown_tool: deny, unknown_method: log_and_pass }
response_filters:
  global:
    strip_ansi_escapes: true
    strip_zero_width: true
    secret_scanners: [aws_access_key, github_token, rsa_private_key]
`

const bundleNoResponseFilters = `
version: 1
server: test
tools: []
defaults: { unknown_tool: allow, unknown_method: log_and_pass }
`

func TestEvaluateResponse_PassthroughWithoutFilters(t *testing.T) {
	b, err := ParseBundle([]byte(bundleNoResponseFilters))
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	e := New(b)
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"\u001b[31mhot\u001b[0m"}]}}`)
	r := e.EvaluateResponse(in)
	if r.Modified {
		t.Error("bundle has no response_filters, payload should pass through unmodified")
	}
	if string(r.Payload) != string(in) {
		t.Errorf("payload changed despite no filters: %q -> %q", in, r.Payload)
	}
}

func TestEvaluateResponse_StripsANSIInMCPContent(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"\u001b[31mhot path\u001b[0m"}]}}`)
	r := e.EvaluateResponse(in)
	if !r.Modified {
		t.Fatal("expected payload modification for ANSI content")
	}
	// Decode the rewritten payload and confirm the ESC bytes are gone but
	// the surrounding structure is intact.
	var msg map[string]any
	if err := json.Unmarshal(r.Payload, &msg); err != nil {
		t.Fatalf("rewritten payload is not valid JSON: %v\nraw: %s", err, r.Payload)
	}
	result := msg["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	if first["text"].(string) != "hot path" {
		t.Errorf("text = %q, want %q", first["text"], "hot path")
	}
	if first["type"].(string) != "text" {
		t.Errorf("type field clobbered: %q", first["type"])
	}
}

func TestEvaluateResponse_RedactsSecretsInDeepStructure(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"rows":[{"col":"AKIAIOSFODNN7EXAMPLE"},{"col":"normal"}]}}`)
	r := e.EvaluateResponse(in)
	if !r.Modified {
		t.Fatal("expected modification for embedded AWS key")
	}
	if strings.Contains(string(r.Payload), "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("AWS key leaked through inspector: %s", r.Payload)
	}
	if !strings.Contains(string(r.Payload), "[REDACTED]") {
		t.Errorf("redaction placeholder missing: %s", r.Payload)
	}
	if len(r.Detections) == 0 {
		t.Fatal("no detections recorded")
	}
}

func TestEvaluateResponse_PreservesIDAndJSONRPCFields(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)

	// Use a string id and an unusually large number to confirm exact
	// round-trip (RawMessage on id avoids float64 precision loss).
	in := []byte(`{"jsonrpc":"2.0","id":"req-9007199254740993","result":{"content":[{"type":"text","text":"AKIAIOSFODNN7EXAMPLE"}]}}`)
	r := e.EvaluateResponse(in)
	if !r.Modified {
		t.Fatal("expected modification")
	}
	if !strings.Contains(string(r.Payload), `"req-9007199254740993"`) {
		t.Errorf("id not preserved verbatim: %s", r.Payload)
	}
	if !strings.Contains(string(r.Payload), `"jsonrpc":"2.0"`) {
		t.Errorf("jsonrpc field missing: %s", r.Payload)
	}
}

func TestEvaluateResponse_CleanPayloadPassesThrough(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)
	in := []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"nothing dangerous here"}]}}`)
	r := e.EvaluateResponse(in)
	if r.Modified {
		t.Errorf("clean payload was modified: %s -> %s", in, r.Payload)
	}
	if len(r.Detections) != 0 {
		t.Errorf("clean payload produced detections: %+v", r.Detections)
	}
}

func TestEvaluateResponse_DoesNotTouchErrorField(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)
	in := []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"AKIAIOSFODNN7EXAMPLE"}}`)
	r := e.EvaluateResponse(in)
	if r.Modified {
		t.Error("error field should not be sanitized in v0.1")
	}
}

func TestEvaluateResponse_NonJSONPassesThrough(t *testing.T) {
	b, _ := ParseBundle([]byte(bundleWithResponseFilters))
	e := New(b)
	in := []byte(`<html>500 internal error</html>`)
	r := e.EvaluateResponse(in)
	if r.Modified {
		t.Error("non-JSON should not be modified")
	}
	if string(r.Payload) != string(in) {
		t.Error("non-JSON payload was altered")
	}
}

func TestParseBundle_RejectsUnknownScanner(t *testing.T) {
	src := `version: 1
server: x
tools: []
response_filters:
  global:
    secret_scanners: [no_such_scanner]
`
	_, err := ParseBundle([]byte(src))
	if err == nil || !strings.Contains(err.Error(), "no_such_scanner") {
		t.Fatalf("expected error about unknown scanner, got %v", err)
	}
}

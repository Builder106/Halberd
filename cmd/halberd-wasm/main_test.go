//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"testing"
)

func TestLoadBundles(t *testing.T) {
	if err := loadBundles(); err != nil {
		t.Fatalf("loadBundles failed: %v", err)
	}

	if len(engines) == 0 {
		t.Fatal("expected loaded engines, got 0")
	}

	expectedPacks := []string{
		"mcp-server-postgres",
		"mcp-server-filesystem",
		"mcp-server-git",
		"mcp-server-github",
		"halberd-honeypot",
	}

	for _, pack := range expectedPacks {
		if _, ok := engines[pack]; !ok {
			t.Errorf("expected engine %q to be loaded", pack)
		}
	}
}

func TestPacksFn(t *testing.T) {
	if err := loadBundles(); err != nil {
		t.Fatalf("loadBundles failed: %v", err)
	}

	res := packsFn(js.Undefined(), nil)
	str, ok := res.(string)
	if !ok {
		t.Fatalf("expected string result from packsFn, got %T", res)
	}

	var packs []struct {
		Name            string `json:"name"`
		Server          string `json:"server"`
		ResponseFilters bool   `json:"responseFilters"`
	}
	if err := json.Unmarshal([]byte(str), &packs); err != nil {
		t.Fatalf("failed to unmarshal packsFn output: %v", err)
	}

	if len(packs) != len(engines) {
		t.Errorf("expected %d packs, got %d", len(engines), len(packs))
	}
}

func TestEvaluateRequestFn(t *testing.T) {
	if err := loadBundles(); err != nil {
		t.Fatalf("loadBundles failed: %v", err)
	}

	t.Run("insufficient args", func(t *testing.T) {
		res := evaluateRequestFn(js.Undefined(), []js.Value{js.ValueOf("mcp-server-postgres")})
		str := res.(string)
		if str != `{"error":"evaluateRequest(pack, payload)"}` {
			t.Errorf("unexpected error payload: %s", str)
		}
	})

	t.Run("unknown pack", func(t *testing.T) {
		res := evaluateRequestFn(js.Undefined(), []js.Value{js.ValueOf("unknown-pack"), js.ValueOf("{}")})
		str := res.(string)
		if str != `{"error":"unknown pack: unknown-pack"}` {
			t.Errorf("unexpected error payload: %s", str)
		}
	})

	t.Run("safe query allowed", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`
		res := evaluateRequestFn(js.Undefined(), []js.Value{js.ValueOf("mcp-server-postgres"), js.ValueOf(payload)})
		str := res.(string)

		var decision struct {
			Blocked    bool `json:"Blocked"`
			Violations any  `json:"Violations"`
		}
		if err := json.Unmarshal([]byte(str), &decision); err != nil {
			t.Fatalf("failed to unmarshal decision: %v", err)
		}
		if decision.Blocked {
			t.Errorf("expected request to be allowed, got blocked: %s", str)
		}
	})

	t.Run("drop table blocked", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`
		res := evaluateRequestFn(js.Undefined(), []js.Value{js.ValueOf("mcp-server-postgres"), js.ValueOf(payload)})
		str := res.(string)

		var decision struct {
			Blocked    bool `json:"Blocked"`
			Violations []struct {
				Rule string `json:"Rule"`
			} `json:"Violations"`
		}
		if err := json.Unmarshal([]byte(str), &decision); err != nil {
			t.Fatalf("failed to unmarshal decision: %v", err)
		}
		if !decision.Blocked {
			t.Errorf("expected request to be blocked, got allowed: %s", str)
		}
	})
}

func TestEvaluateResponseFn(t *testing.T) {
	if err := loadBundles(); err != nil {
		t.Fatalf("loadBundles failed: %v", err)
	}

	t.Run("insufficient args", func(t *testing.T) {
		res := evaluateResponseFn(js.Undefined(), []js.Value{})
		str := res.(string)
		if str != `{"error":"evaluateResponse(pack, payload)"}` {
			t.Errorf("unexpected error payload: %s", str)
		}
	})

	t.Run("unknown pack", func(t *testing.T) {
		res := evaluateResponseFn(js.Undefined(), []js.Value{js.ValueOf("unknown-pack"), js.ValueOf("{}")})
		str := res.(string)
		if str != `{"error":"unknown pack: unknown-pack"}` {
			t.Errorf("unexpected error payload: %s", str)
		}
	})

	t.Run("clean response unmodified", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"hello world"}]}}`
		res := evaluateResponseFn(js.Undefined(), []js.Value{js.ValueOf("mcp-server-postgres"), js.ValueOf(payload)})
		str := res.(string)

		var out struct {
			Modified   bool   `json:"modified"`
			Payload    string `json:"payload"`
			Detections []any  `json:"detections"`
		}
		if err := json.Unmarshal([]byte(str), &out); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if out.Modified {
			t.Errorf("expected modified=false, got true")
		}
	})

	t.Run("secret response modified", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"key: AKIAIOSFODNN7EXAMPLE"}]}}`
		res := evaluateResponseFn(js.Undefined(), []js.Value{js.ValueOf("mcp-server-postgres"), js.ValueOf(payload)})
		str := res.(string)

		var out struct {
			Modified   bool `json:"modified"`
			Payload    string
			Detections []struct {
				Kind string `json:"kind"`
			} `json:"detections"`
		}
		if err := json.Unmarshal([]byte(str), &out); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if !out.Modified {
			t.Errorf("expected modified=true, got false")
		}
		if len(out.Detections) == 0 || out.Detections[0].Kind != "aws_access_key" {
			t.Errorf("expected aws_access_key detection, got %+v", out.Detections)
		}
	})
}

func TestJsErr(t *testing.T) {
	res := jsErr("test error message")
	expected := `{"error":"test error message"}`
	if fmt.Sprintf("%v", res) != expected {
		t.Errorf("expected %s, got %v", expected, res)
	}
}

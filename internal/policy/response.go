package policy

import (
	"encoding/json"
	"fmt"
)

// Detection is one rewrite the response inspector made: which threat class
// it matched and where in the JSON tree it lived. The original secret or
// escape sequence is intentionally NOT included — we don't want the audit
// log to leak the very content the filter was meant to suppress.
type Detection struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// ResponseResult is the outcome of evaluating one JSON-RPC response payload.
// Payload is the bytes the transport should forward to the agent: equal to
// the input bytes if Modified is false, a re-serialized envelope with
// sanitized string leaves if Modified is true.
type ResponseResult struct {
	Payload    []byte
	Modified   bool
	Detections []Detection
}

// EvaluateResponse inspects a JSON-RPC response payload and returns either
// the original bytes (when no response filters are configured, when the
// payload doesn't parse as JSON-RPC, or when nothing matched) or a
// re-serialized envelope with sanitized string leaves.
//
// EvaluateResponse only touches the `result` subtree. The envelope's
// `jsonrpc`, `id`, and `error` fields round-trip exactly (via
// json.RawMessage) so the agent's MCP client doesn't see drift on
// protocol-level metadata.
func (e *Engine) EvaluateResponse(payload []byte) ResponseResult {
	if e.bundle.ResponseFilters == nil {
		return ResponseResult{Payload: payload}
	}

	var env struct {
		JSONRPC string          `json:"jsonrpc,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   json.RawMessage `json:"error,omitempty"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		// Not a JSON-RPC envelope (e.g. a transport-level HTML error page).
		// Forward as-is; the proxy is not in the business of mangling
		// non-protocol traffic.
		return ResponseResult{Payload: payload}
	}
	if len(env.Result) == 0 {
		return ResponseResult{Payload: payload}
	}

	newResult, detections, modified := sanitizeJSONNode(env.Result, "result", &e.bundle.ResponseFilters.Global)
	if !modified {
		return ResponseResult{Payload: payload, Detections: detections}
	}

	env.Result = newResult
	out, _ := json.Marshal(env)
	return ResponseResult{Payload: out, Modified: true, Detections: detections}
}

// sanitizeJSONNode walks a JSON value recursively. String leaves are
// sanitized; arrays and objects are descended into. Numbers, booleans,
// and nulls are returned unchanged. The path argument accumulates a
// human-readable location like `result.content[0].text` for audit
// entries.
func sanitizeJSONNode(raw json.RawMessage, path string, filter *GlobalResponseFilter) (json.RawMessage, []Detection, bool) {
	// Try string first (cheapest unmarshal, hot path for content leaves).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		newS, detections := filter.sanitizeString(s, path)
		if newS == s {
			return raw, nil, false
		}
		out, _ := json.Marshal(newS)
		return out, detections, true
	}

	// Try array.
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		var allDet []Detection
		modified := false
		for i := range arr {
			newItem, det, mod := sanitizeJSONNode(arr[i], fmt.Sprintf("%s[%d]", path, i), filter)
			arr[i] = newItem
			allDet = append(allDet, det...)
			if mod {
				modified = true
			}
		}
		if !modified {
			return raw, allDet, false
		}
		out, _ := json.Marshal(arr)
		return out, allDet, true
	}

	// Try object.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		var allDet []Detection
		modified := false
		for k, v := range obj {
			newVal, det, mod := sanitizeJSONNode(v, path+"."+k, filter)
			obj[k] = newVal
			allDet = append(allDet, det...)
			if mod {
				modified = true
			}
		}
		if !modified {
			return raw, allDet, false
		}
		out, _ := json.Marshal(obj)
		return out, allDet, true
	}

	// Number, bool, null — nothing to sanitize.
	return raw, nil, false
}

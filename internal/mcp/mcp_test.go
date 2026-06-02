package mcp

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeSetStatus(t *testing.T) {
	var gotState string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotState, _ = body["state"].(string)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	t.Setenv("ORCHID_SESSION", "s-1")
	t.Setenv("ORCHID_URL", srv.URL)

	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"set_status","arguments":{"state":"requires_action"}}}`,
	}, "\n") + "\n"

	var out strings.Builder
	if err := Serve(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	if gotState != "requires_action" {
		t.Fatalf("server received state %q", gotState)
	}
	sc := bufio.NewScanner(strings.NewReader(out.String()))
	ids := map[float64]bool{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var resp map[string]any
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response not JSON: %q", line)
		}
		if id, ok := resp["id"].(float64); ok {
			ids[id] = true
		}
	}
	for _, want := range []float64{1, 2, 3} {
		if !ids[want] {
			t.Fatalf("missing response for id %v in:\n%s", want, out.String())
		}
	}
}

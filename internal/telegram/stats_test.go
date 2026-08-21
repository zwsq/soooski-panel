package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatsCounterDeltas(t *testing.T) {
	n := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		n++
		users := map[string]any{
			"u9": map[string]int64{"bytes_in": 100, "bytes_out": 400},
		}
		if n >= 2 {
			users["u9"] = map[string]int64{"bytes_in": 150, "bytes_out": 450}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"users": users})
	}))
	t.Cleanup(ts.Close)
	c := NewStatsCounter()
	c.URL = ts.URL + "/stats"
	first, err := c.Poll()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 {
		t.Fatalf("baseline should not charge: %#v", first)
	}
	second, err := c.Poll()
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != 9 || second[0].Up != 50 || second[0].Down != 50 {
		t.Fatalf("delta %#v", second)
	}
}

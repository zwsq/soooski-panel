package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompileForcesIPv4ForDestDials(t *testing.T) {
	raw, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		`"strategy": "ipv4_only"`,
		`"domain_strategy": "ipv4_only"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in\n%s", want, s)
		}
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	n := strings.Count(s, `"domain_strategy": "ipv4_only"`)
	if n < 3 {
		// direct outbound + REALITY handshake + ShadowTLS handshake
		t.Fatalf("expected ipv4_only on dest dialers, found %d", n)
	}
}

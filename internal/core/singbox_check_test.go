package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/zwsq/soooski-panel/internal/crypto"
)

func TestSingBoxCheckAcceptsCompiledConfig(t *testing.T) {
	bin := ""
	for _, c := range []string{os.Getenv("SINGBOX_BIN"), "/tmp/sing-box", "sing-box"} {
		if c == "" {
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			bin = p
			break
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			bin = c
			break
		}
	}
	if bin == "" {
		t.Skip("sing-box binary not available")
	}
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	key := filepath.Join(dir, "server.key")
	if err := crypto.EnsureSelfSigned(cert, key, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	in := sampleInput()
	priv, pub, err := crypto.RealityKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	wpriv, wpub, err := crypto.WireGuardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	upriv, upub, err := crypto.WireGuardKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	in.Settings.RealityPrivateKey = priv
	in.Settings.RealityPublicKey = pub
	in.Settings.WGPrivateKey = wpriv
	in.Settings.WGPublicKey = wpub
	in.Settings.TLSCertPath = cert
	in.Settings.TLSKeyPath = key
	in.Settings.SSPassword = crypto.SS2022Password()
	in.Users[0].SSPassword = crypto.SS2022Password()
	in.Users[0].WGPrivateKey = upriv
	in.Users[0].WGPublicKey = upub
	raw, err := Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "sing-box.json")
	if err := os.WriteFile(cfg, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "check", "-c", cfg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check: %v\n%s\nconfig:\n%s", err, out, raw)
	}
}

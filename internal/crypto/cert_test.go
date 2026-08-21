package crypto

import (
	"path/filepath"
	"testing"
)

func TestSelfSignedIsNotPublic(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "c.pem")
	key := filepath.Join(dir, "k.pem")
	if err := EnsureSelfSigned(cert, key, "vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	if PublicCertTrusts(cert, "vpn.example.com") {
		t.Fatal("self-signed must not count as a public cert")
	}
	info, err := ReadLeafCert(cert)
	if err != nil || !info.SelfSigned {
		t.Fatalf("%+v %v", info, err)
	}
}

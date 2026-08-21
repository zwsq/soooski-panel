package main

import (
	"testing"

	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/store"
)

func TestResetAdminCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SOOOSKI_DATA_DIR", dir)
	t.Setenv("SOOOSKI_ADMIN_USER", "admin")
	t.Setenv("SOOOSKI_ADMIN_PASSWORD", "oldpass12")
	st, _, err := store.Open(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	if err := resetAdmin([]string{"-user", "root", "-password", "newpass12"}); err != nil {
		t.Fatal(err)
	}
	st, _, err = store.Open(config.Load())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	a, err := st.AdminByUsername("root")
	if err != nil || !crypto.CheckPassword(a.PasswordHash, "newpass12") {
		t.Fatalf("%#v %v", a, err)
	}
	if runCLI([]string{"nope"}) != 2 {
		t.Fatal("unknown command should not start the server")
	}
	if runCLI([]string{"help"}) != 0 {
		t.Fatal("help")
	}
}

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/store"
)

func usage(w io.Writer) {
	fmt.Fprint(w, `soooski — panel process inside the container.

Usage:
  soooski                 start the panel (docker entrypoint)
  soooski reset-admin     set admin username/password; drop login sessions
  soooski help

reset-admin flags:
  -user string       admin username (default: keep current)
  -password string   new password (default: generate and print once)

On the VPS use the host CLI (soooski reset-admin), which calls this for you.
`)
}

func runCLI(args []string) int {
	if len(args) == 0 {
		runServer()
		return 0
	}
	switch args[0] {
	case "serve", "run":
		runServer()
		return 0
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	case "reset-admin":
		if err := resetAdmin(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "reset-admin: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func resetAdmin(args []string) error {
	fs := flag.NewFlagSet("reset-admin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	user := fs.String("user", "", "admin username (default: keep current)")
	pass := fs.String("password", "", "new password (default: generate)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	username := strings.TrimSpace(*user)
	password := *pass
	generated := false
	if password == "" {
		password = crypto.RandomPassword(16)
		generated = true
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if username != "" {
		if err := validateAdminUsername(username); err != nil {
			return err
		}
	}
	cfg := config.Load()
	st, _, err := store.Open(cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	admin, err := st.ResetAdmin(username, hash)
	if err != nil {
		return err
	}
	fmt.Println("admin user:", admin.Username)
	if generated {
		fmt.Println("admin pass:", password)
		fmt.Println("(shown once — this reset dropped existing login sessions)")
	} else {
		fmt.Println("password updated; login sessions dropped")
	}
	return nil
}

func validateAdminUsername(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must be 3-64 characters")
	}
	for _, c := range username {
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
		if !ok {
			return fmt.Errorf("username may contain letters, digits, . _ -")
		}
	}
	return nil
}

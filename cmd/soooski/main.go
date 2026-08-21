package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/zwsq/soooski-panel/internal/api"
	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/core"
	"github.com/zwsq/soooski-panel/internal/ingress"
	"github.com/zwsq/soooski-panel/internal/store"
	"github.com/zwsq/soooski-panel/internal/supervisor"
	"github.com/zwsq/soooski-panel/internal/telegram"
	"github.com/zwsq/soooski-panel/internal/tlsutil"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatal(err)
	}
	st, bootstrapPass, err := store.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	sup := supervisor.New(cfg.SingBoxBin, cfg.SingBoxConfigPath(), cfg.CoreMode, os.Stderr)
	tg := telegram.New(cfg.MtgBin, cfg.MtgConfigPath(), cfg.CoreMode, os.Stderr)
	apply := func() error {
		settings, users, _, inbounds, err := st.Snapshot()
		if err != nil {
			return err
		}
		raw, err := core.Compile(core.CompileInput{Settings: settings, Users: users, Inbounds: inbounds})
		if err != nil {
			return err
		}
		if err := sup.Apply(raw); err != nil {
			return err
		}
		if err := tg.Apply(settings, users); err != nil {
			log.Printf("telegram: %v", err)
		}
		return nil
	}
	if err := apply(); err != nil {
		log.Printf("core apply: %v", err)
	}

	apiSrv := &api.Server{
		Store:      st,
		Supervisor: sup,
		Apply:      apply,
		DataDir:    cfg.DataDir,
	}
	ing := ingress.New(apiSrv.Handler(), func() []ingress.Route {
		inbounds, err := st.Inbounds()
		if err != nil {
			return nil
		}
		return ingress.RoutesFrom(inbounds)
	})
	ing.AdminPrefix = func() string {
		s, _ := st.Settings()
		return s.AdminPrefix()
	}
	ing.ClientPrefix = func() string {
		s, _ := st.Settings()
		return s.ClientPrefix()
	}
	ing.HostsFn = func() []string {
		s, err := st.Settings()
		if err != nil {
			return nil
		}
		out := []string{s.PublicHost}
		domains, err := st.Domains()
		if err != nil {
			return out
		}
		for _, d := range domains {
			if d.Enable {
				out = append(out, d.Domain)
			}
		}
		return out
	}
	ing.RealityAddr = "127.0.0.1:12001"
	ing.TelegramFn = func() (string, string, bool) {
		s, err := st.Settings()
		if err != nil || !s.TelegramEnabled {
			return "", "", false
		}
		d := strings.TrimSpace(s.TelegramFakeDomain)
		if d == "" {
			return "", "", false
		}
		return d, telegram.ListenAddr, true
	}

	settings, err := st.Settings()
	if err != nil {
		log.Fatal(err)
	}
	acmeEmail := settings.ACMEEmail
	if acmeEmail == "" {
		acmeEmail = cfg.ACMEEmail
	}
	tlsMgr, err := tlsutil.New(acmeEmail, cfg.ACMECacheDir(), cfg.TLSCertPath(), cfg.TLSKeyPath(), ing.HostsFn)
	if err != nil {
		log.Fatal(err)
	}
	apiSrv.TLSEmail = tlsMgr.SetEmail
	apiSrv.Certs = tlsMgr.Status
	apiSrv.IssueCerts = func() error {
		return tlsMgr.IssueAll(context.Background())
	}
	tlsMgr.SetOnChange(func() { _ = apply() })
	ing.Challenge = tlsMgr.ServeChallenge
	go tlsMgr.Maintain(context.Background())
	counter := supervisor.NewCounter(settings.ClashSecret)
	tgTraffic := telegram.NewStatsCounter()
	apiSrv.TrafficErr = func() string {
		if e := counter.LastError(); e != "" {
			return e
		}
		return tgTraffic.LastError()
	}
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for range t.C {
			var deltas []supervisor.TrafficDelta
			if d, err := counter.Poll(); err == nil {
				deltas = append(deltas, d...)
			}
			if d, err := tgTraffic.Poll(); err == nil {
				for _, x := range d {
					deltas = append(deltas, supervisor.TrafficDelta{ID: x.ID, Up: x.Up, Down: x.Down})
				}
			}
			if len(deltas) == 0 {
				continue
			}
			users, err := st.Users()
			if err != nil {
				continue
			}
			idx := map[string]int64{}
			for _, u := range users {
				idx[u.Username] = u.ID
			}
			changed := false
			for _, d := range deltas {
				id := d.ID
				if id == 0 {
					var ok bool
					id, ok = idx[d.User]
					if !ok {
						continue
					}
				}
				before, err := st.UserByID(id)
				if err != nil {
					continue
				}
				wasActive := before.Active()
				_ = st.AddTraffic(id, d.Up, d.Down)
				after, err := st.UserByID(id)
				if err != nil {
					continue
				}
				if wasActive != after.Active() {
					changed = true
				}
			}
			if changed {
				_ = apply()
			}
		}
	}()

	plain := &http.Server{Addr: cfg.ListenHTTP, Handler: tlsMgr.HTTPHandler(ing)}
	errCh := make(chan error, 2)
	go func() { errCh <- listenOptional(plain) }()
	go func() { errCh <- ing.ServeSNI(cfg.ListenHTTPS, tlsMgr.TLSConfig()) }()

	host := settings.PublicHost
	if host == "" {
		host = "<server-ip-or-domain>"
	}
	adminURL := fmt.Sprintf("https://%s%s/", host, settings.AdminPrefix())
	_ = os.WriteFile(filepath.Join(cfg.DataDir, "admin-url.txt"), []byte(adminURL+"\n"), 0600)

	fmt.Println("========================================")
	fmt.Println(" soooski")
	fmt.Println(" data:       ", cfg.DataDir)
	fmt.Println(" admin URL:  ", adminURL)
	fmt.Println("             ", fmt.Sprintf("http://%s%s/", host, settings.AdminPrefix()))
	fmt.Println(" client path:", settings.ClientPrefix()+"/<user-token>")
	if tlsutil.Eligible(host) {
		fmt.Println(" https:      Let's Encrypt for", host, "(HTTP-01 on :80; self-signed until the cert is issued)")
		if acmeEmail == "" {
			fmt.Println("             set SOOOSKI_ACME_EMAIL (or Settings) so Let's Encrypt can reach you")
		}
	} else {
		fmt.Println(" https:      self-signed (set SOOOSKI_PUBLIC_HOST to a real hostname for Let's Encrypt)")
	}
	fmt.Println(" (bookmark the admin URL — secret path, same idea as Hiddify)")
	if bootstrapPass != "" {
		fmt.Println(" user:       ", cfg.AdminUser)
		fmt.Println(" pass:       ", bootstrapPass)
		fmt.Println(" (shown once — set SOOOSKI_ADMIN_PASSWORD to choose it)")
	}
	fmt.Println("========================================")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = plain.Shutdown(ctx)
	sup.Stop()
	tg.Stop()
}

func listenOptional(s *http.Server) error {
	err := s.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("listen %s: %v", s.Addr, err)
	}
	return err
}

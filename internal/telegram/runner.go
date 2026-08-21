package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/zwsq/soooski-panel/internal/models"
)

type Runner struct {
	Bin        string
	ConfigPath string
	Mode       string // embed | noop
	Log        *log.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	running bool
	lastErr string
	lastCfg []byte
}

func New(bin, configPath, mode string, w io.Writer) *Runner {
	if w == nil {
		w = os.Stderr
	}
	return &Runner{
		Bin:        bin,
		ConfigPath: configPath,
		Mode:       mode,
		Log:        log.New(w, "mtg ", log.LstdFlags),
	}
}

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) LastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

func (r *Runner) Apply(st models.Settings, users []models.User) error {
	if !st.TelegramEnabled {
		r.Stop()
		return nil
	}
	raw := []byte(RenderTOML(st, users))
	r.mu.Lock()
	same := r.running && bytes.Equal(r.lastCfg, raw)
	r.mu.Unlock()
	if same {
		return nil
	}
	if err := os.WriteFile(r.ConfigPath, raw, 0600); err != nil {
		return err
	}
	if r.Mode == "noop" {
		r.mu.Lock()
		r.running = true
		r.lastErr = ""
		r.lastCfg = raw
		r.mu.Unlock()
		return nil
	}
	if _, err := exec.LookPath(r.Bin); err != nil {
		msg := fmt.Sprintf("mtg-multi binary not found (%s); Telegram proxy is off", r.Bin)
		r.mu.Lock()
		r.lastErr = msg
		r.running = false
		r.mu.Unlock()
		return errors.New(msg)
	}
	if r.Running() && r.postReload() == nil {
		r.mu.Lock()
		r.lastCfg = raw
		r.mu.Unlock()
		return nil
	}
	if err := r.restart(); err != nil {
		return err
	}
	r.mu.Lock()
	r.lastCfg = raw
	r.mu.Unlock()
	return nil
}

func (r *Runner) postReload() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ReloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mtg reload %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (r *Runner) restart() error {
	r.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, r.Bin, "run", r.ConfigPath)
	cmd.Stdout = r.Log.Writer()
	cmd.Stderr = r.Log.Writer()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		cancel()
		r.mu.Lock()
		r.lastErr = err.Error()
		r.running = false
		r.mu.Unlock()
		return err
	}
	r.mu.Lock()
	r.cmd = cmd
	r.cancel = cancel
	r.running = true
	r.lastErr = ""
	r.mu.Unlock()
	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		if r.cmd == cmd {
			r.running = false
			if err != nil && ctx.Err() == nil {
				r.lastErr = err.Error()
				r.Log.Printf("exited: %v", err)
			}
		}
		r.mu.Unlock()
	}()
	time.Sleep(150 * time.Millisecond)
	return nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	cancel := r.cancel
	r.cmd = nil
	r.cancel = nil
	r.running = false
	r.lastCfg = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _, _ = cmd.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}

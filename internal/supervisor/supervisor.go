package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Supervisor struct {
	Bin        string
	ConfigPath string
	Mode       string // embed | noop
	Log        *log.Logger

	mu      sync.Mutex
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	running bool
	lastErr string
}

func New(bin, configPath, mode string, w io.Writer) *Supervisor {
	if w == nil {
		w = os.Stderr
	}
	return &Supervisor{
		Bin:        bin,
		ConfigPath: configPath,
		Mode:       mode,
		Log:        log.New(w, "sing-box ", log.LstdFlags),
	}
}

func (s *Supervisor) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Supervisor) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

func (s *Supervisor) Apply(cfg []byte) error {
	if err := os.WriteFile(s.ConfigPath, cfg, 0600); err != nil {
		return err
	}
	if s.Mode == "noop" {
		s.mu.Lock()
		s.running = true
		s.lastErr = ""
		s.mu.Unlock()
		return nil
	}
	if _, err := exec.LookPath(s.Bin); err != nil {
		s.mu.Lock()
		s.lastErr = fmt.Sprintf("sing-box binary not found (%s); panel is up, data plane is not", s.Bin)
		s.running = false
		s.mu.Unlock()
		return errors.New(s.lastErr)
	}
	check := exec.Command(s.Bin, "check", "-c", s.ConfigPath)
	if out, err := check.CombinedOutput(); err != nil {
		msg := fmt.Sprintf("sing-box check: %v: %s", err, out)
		s.mu.Lock()
		s.lastErr = msg
		s.mu.Unlock()
		return errors.New(msg)
	}
	return s.restart()
}

func (s *Supervisor) restart() error {
	s.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, s.Bin, "run", "-c", s.ConfigPath)
	cmd.Stdout = s.Log.Writer()
	cmd.Stderr = s.Log.Writer()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		cancel()
		s.mu.Lock()
		s.lastErr = err.Error()
		s.running = false
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.cmd = cmd
	s.cancel = cancel
	s.running = true
	s.lastErr = ""
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.running = false
			if err != nil && ctx.Err() == nil {
				s.lastErr = err.Error()
				s.Log.Printf("exited: %v", err)
			}
		}
		s.mu.Unlock()
	}()
	time.Sleep(150 * time.Millisecond)
	return nil
}

func (s *Supervisor) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	cancel := s.cancel
	s.cmd = nil
	s.cancel = nil
	s.running = false
	s.mu.Unlock()
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

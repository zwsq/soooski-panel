package ingress

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type prefixConn struct {
	net.Conn
	r io.Reader
}

func (c *prefixConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func peekSNI(c net.Conn, timeout time.Duration) (sni string, wrapped net.Conn, err error) {
	_ = c.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = c.SetReadDeadline(time.Time{}) }()
	hdr := make([]byte, 5)
	if _, err = io.ReadFull(c, hdr); err != nil {
		return "", nil, err
	}
	if hdr[0] != 0x16 {
		wrapped = &prefixConn{Conn: c, r: io.MultiReader(bytes.NewReader(hdr), c)}
		return "", wrapped, nil
	}
	n := int(binary.BigEndian.Uint16(hdr[3:5]))
	if n < 0 || n > 16*1024 {
		return "", nil, errors.New("bad tls record")
	}
	rest := make([]byte, n)
	if _, err = io.ReadFull(c, rest); err != nil {
		return "", nil, err
	}
	rec := append(hdr, rest...)
	sni = sniFromHandshake(rest)
	wrapped = &prefixConn{Conn: c, r: io.MultiReader(bytes.NewReader(rec), c)}
	return sni, wrapped, nil
}

func sniFromHandshake(msg []byte) string {
	if len(msg) < 42 || msg[0] != 0x01 {
		return ""
	}
	// handshake header 4 + client_version 2 + random 32 + session_id
	off := 4 + 2 + 32
	if off >= len(msg) {
		return ""
	}
	sidLen := int(msg[off])
	off += 1 + sidLen
	if off+2 > len(msg) {
		return ""
	}
	csLen := int(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2 + csLen
	if off >= len(msg) {
		return ""
	}
	compLen := int(msg[off])
	off += 1 + compLen
	if off+2 > len(msg) {
		return ""
	}
	extLen := int(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2
	end := off + extLen
	if end > len(msg) {
		end = len(msg)
	}
	for off+4 <= end {
		typ := binary.BigEndian.Uint16(msg[off : off+2])
		l := int(binary.BigEndian.Uint16(msg[off+2 : off+4]))
		off += 4
		if off+l > end {
			break
		}
		if typ == 0 && l >= 5 {
			list := msg[off : off+l]
			if len(list) < 3 {
				break
			}
			nameOff := 2
			if nameOff+3 > len(list) {
				break
			}
			if list[nameOff] != 0 {
				break
			}
			nl := int(binary.BigEndian.Uint16(list[nameOff+1 : nameOff+3]))
			nameOff += 3
			if nameOff+nl > len(list) {
				break
			}
			return string(list[nameOff : nameOff+nl])
		}
		off += l
	}
	return ""
}

func proxyTCP(client net.Conn, addr string) {
	defer client.Close()
	backend, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return
	}
	defer backend.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(backend, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, backend); done <- struct{}{} }()
	<-done
}

type chanListener struct {
	addr net.Addr
	ch   chan net.Conn
	once sync.Once
	done chan struct{}
}

func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-l.ch:
		if !ok {
			return nil, net.ErrClosed
		}
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *chanListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *chanListener) Addr() net.Addr { return l.addr }

func hostMatch(sni string, hosts []string) bool {
	sni = strings.ToLower(strings.TrimSpace(sni))
	if sni == "" {
		return false
	}
	for _, h := range hosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if sni == h {
			return true
		}
	}
	return false
}

const LocalHTTPS = "local"

func (i *Ingress) telegramAddr(sni string) string {
	if i.TelegramFn == nil {
		return ""
	}
	fake, addr, ok := i.TelegramFn()
	if !ok || addr == "" || fake == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(sni), strings.TrimSpace(fake)) {
		return addr
	}
	return ""
}

// RouteSNI is the TCP backend for a ClientHello SNI. LocalHTTPS means the
// panel / path-mux HTTPS listener; anything else is dialed as host:port.
func (i *Ingress) RouteSNI(sni string) string {
	if i.Ours(sni) {
		return LocalHTTPS
	}
	if addr := i.telegramAddr(sni); addr != "" {
		return addr
	}
	if i.RealityAddr != "" {
		return i.RealityAddr
	}
	return "127.0.0.1:12001"
}

func (i *Ingress) Ours(sni string) bool {
	sni = strings.ToLower(strings.TrimSpace(sni))
	// Browsers hitting https://IP/ send the IP as SNI. Empty SNI is not a
	// REALITY client (those always send the dest name).
	if sni == "" {
		return true
	}
	if ip := net.ParseIP(sni); ip != nil {
		return true
	}
	if i.HostsFn == nil {
		return false
	}
	return hostMatch(sni, i.HostsFn())
}

// ServeSNI listens on TCP addr, sends our-domain (and IP/empty SNI) handshakes
// to the HTTPS handler (panel + CDN paths), Telegram FakeTLS SNI to mtg, and
// every other SNI to REALITY.
func (i *Ingress) ServeSNI(addr string, tlsCfg *tls.Config) error {
	if tlsCfg == nil {
		return errors.New("tls config required")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	ch := make(chan net.Conn, 16)
	cl := &chanListener{addr: ln.Addr(), ch: ch, done: make(chan struct{})}
	cfg := tlsCfg.Clone()
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS12
	}
	if len(cfg.NextProtos) == 0 {
		cfg.NextProtos = []string{"h2", "http/1.1"}
	}
	srv := &http.Server{Handler: i, TLSConfig: cfg}
	go func() { _ = srv.Serve(tls.NewListener(cl, cfg)) }()
	reality := i.RealityAddr
	if reality == "" {
		reality = "127.0.0.1:12001"
	}
	for {
		c, err := ln.Accept()
		if err != nil {
			_ = cl.Close()
			return err
		}
		go func(c net.Conn) {
			sni, wc, err := peekSNI(c, 8*time.Second)
			if err != nil {
				_ = c.Close()
				return
			}
			dest := i.RouteSNI(sni)
			if dest == LocalHTTPS {
				select {
				case ch <- wc:
				case <-cl.done:
					_ = wc.Close()
				}
				return
			}
			if dest == "" {
				dest = reality
			}
			proxyTCP(wc, dest)
		}(c)
	}
}

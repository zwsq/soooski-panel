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

func innerTCP(c net.Conn) *net.TCPConn {
	for c != nil {
		switch t := c.(type) {
		case *net.TCPConn:
			return t
		case *prefixConn:
			c = t.Conn
		default:
			return nil
		}
	}
	return nil
}

const maxTLSRecord = 16 * 1024

func setTCPOpts(c net.Conn) {
	t := innerTCP(c)
	if t == nil {
		return
	}
	_ = t.SetNoDelay(true)
	_ = t.SetKeepAlive(true)
	_ = t.SetKeepAlivePeriod(30 * time.Second)
}

func peekSNIFromBuf(buf []byte, n int) (sni string, need int, ok bool) {
	if n < 5 {
		return "", 5, false
	}
	if buf[0] != 0x16 {
		return "", 0, true
	}
	recLen := int(binary.BigEndian.Uint16(buf[3:5]))
	if recLen < 0 || recLen > maxTLSRecord {
		return "", 0, true
	}
	need = 5 + recLen
	if n < need {
		return "", need, false
	}
	return sniFromHandshake(buf[5:need]), 0, true
}

// peekSNI reads the first TLS record to get SNI, then replays those exact
// bytes to the backend. MSG_PEEK + Go's edge-triggered poller can stall or
// truncate the ClientHello; REALITY HMACs the bytes, so a faithful replay
// is required. Dest overflow (www.microsoft.com) is a separate issue.
func peekSNI(c net.Conn, timeout time.Duration) (sni string, wrapped net.Conn, err error) {
	_ = c.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = c.SetReadDeadline(time.Time{}) }()
	return peekSNIConsume(c)
}

func peekSNIConsume(c net.Conn) (sni string, wrapped net.Conn, err error) {
	hdr := make([]byte, 5)
	if _, err = io.ReadFull(c, hdr); err != nil {
		return "", nil, err
	}
	if hdr[0] != 0x16 {
		wrapped = &prefixConn{Conn: c, r: io.MultiReader(bytes.NewReader(hdr), c)}
		return "", wrapped, nil
	}
	n := int(binary.BigEndian.Uint16(hdr[3:5]))
	if n < 0 || n > maxTLSRecord {
		return "", nil, errors.New("bad tls record")
	}
	rest := make([]byte, n)
	if _, err = io.ReadFull(c, rest); err != nil {
		return "", nil, err
	}
	rec := make([]byte, 0, 5+len(rest))
	rec = append(rec, hdr...)
	rec = append(rec, rest...)
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
	setTCPOpts(client)
	backend, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return
	}
	defer backend.Close()
	setTCPOpts(backend)
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(backend, client)
		if t := innerTCP(backend); t != nil {
			_ = t.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, backend)
		if t := innerTCP(client); t != nil {
			_ = t.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
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

func (i *Ingress) realityAddr() string {
	if i != nil && i.RealityAddr != "" {
		return i.RealityAddr
	}
	return "127.0.0.1:12001"
}

// RouteSNI is the TCP backend for a ClientHello SNI. LocalHTTPS means the
// panel / path-mux HTTPS listener; anything else is dialed as host:port.
// Unknown names go to REALITY (same as HAProxy ssl_preread). Sending them to
// the panel presents our cert; REALITY clients then log tls: bad certificate.
func (i *Ingress) RouteSNI(sni string) string {
	if addr := i.telegramAddr(sni); addr != "" {
		return addr
	}
	if i.Ours(sni) {
		return LocalHTTPS
	}
	return i.realityAddr()
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
			if dest == LocalHTTPS || dest == "" {
				select {
				case ch <- wc:
				case <-cl.done:
					_ = wc.Close()
				}
				return
			}
			proxyTCP(wc, dest)
		}(c)
	}
}

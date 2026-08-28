package ingress

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func tlsClientHello(sni string) []byte {
	var hello []byte
	hello = append(hello, 0x01)             // handshake type ClientHello
	hello = append(hello, 0, 0, 0)          // length placeholder
	hello = append(hello, 0x03, 0x03)       // version TLS 1.2
	hello = append(hello, make([]byte, 32)...)
	hello = append(hello, 0)                // session id length
	hello = append(hello, 0, 2, 0x13, 0x01) // one cipher suite
	hello = append(hello, 1, 0)             // compression
	var ext []byte
	host := []byte(sni)
	name := []byte{0} // host_name
	name = binary.BigEndian.AppendUint16(name, uint16(len(host)))
	name = append(name, host...)
	list := binary.BigEndian.AppendUint16(nil, uint16(len(name)))
	list = append(list, name...)
	ext = binary.BigEndian.AppendUint16(ext, 0) // SNI type
	ext = binary.BigEndian.AppendUint16(ext, uint16(len(list)))
	ext = append(ext, list...)
	hello = binary.BigEndian.AppendUint16(hello, uint16(len(ext)))
	hello = append(hello, ext...)
	binary.BigEndian.PutUint32(hello[0:4], uint32(len(hello)-4))
	hello[0] = 0x01
	rec := []byte{0x16, 0x03, 0x03, 0, 0}
	binary.BigEndian.PutUint16(rec[3:5], uint16(len(hello)))
	return append(rec, hello...)
}

func TestPeekSNIFromBuf(t *testing.T) {
	raw := tlsClientHello("www.microsoft.com")
	sni, _, ok := peekSNIFromBuf(raw, len(raw))
	if !ok || sni != "www.microsoft.com" {
		t.Fatalf("sni %q ok=%v", sni, ok)
	}
	if _, need, ok := peekSNIFromBuf(raw[:3], 3); ok || need != 5 {
		t.Fatalf("partial header need=%d ok=%v", need, ok)
	}
}

func TestPeekSNIDoesNotConsumeTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	hello := tlsClientHello("www.microsoft.com")
	got := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			got <- err
			return
		}
		defer c.Close()
		sni, wc, err := peekSNI(c, 2*time.Second)
		if err != nil {
			got <- err
			return
		}
		if sni != "www.microsoft.com" {
			got <- errString("sni " + sni)
			return
		}
		buf := make([]byte, len(hello))
		if _, err := io.ReadFull(wc, buf); err != nil {
			got <- err
			return
		}
		if string(buf) != string(hello) {
			got <- errString("clienthello bytes changed after peek")
			return
		}
		got <- nil
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write(hello); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/curve25519"
)

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func RandomHex(nBytes int) string {
	return hex.EncodeToString(RandomBytes(nBytes))
}

func RandomToken() string {
	return RandomHex(16)
}

func NewUUID() string {
	return uuid.NewString()
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// RealityKeyPair returns X25519 keys encoded the way sing-box / Xray expect.
func RealityKeyPair() (privateKey, publicKey string, err error) {
	var priv [32]byte
	if _, err = rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(priv[:]),
		base64.RawURLEncoding.EncodeToString(pub), nil
}

func ShortID() string {
	return hex.EncodeToString(RandomBytes(4))
}

func SS2022Password() string {
	return base64.StdEncoding.EncodeToString(RandomBytes(16))
}

func WireGuardKeyPair() (privateKey, publicKey string, err error) {
	var priv [32]byte
	if _, err = rand.Read(priv[:]); err != nil {
		return "", "", err
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(priv[:]),
		base64.StdEncoding.EncodeToString(pub), nil
}

func SecretSegment(n int) string {
	return RandomPassword(n)
}

// TelegramFakeTLSSecret is mtg / Hiddify FakeTLS: ee + 16 random bytes + hex(hostname).
func TelegramFakeTLSSecret(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return "ee" + hex.EncodeToString(RandomBytes(16)) + hex.EncodeToString([]byte(domain))
}

func TelegramSecretMatchesDomain(secret, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	want := hex.EncodeToString([]byte(domain))
	secret = strings.ToLower(strings.TrimSpace(secret))
	return strings.HasPrefix(secret, "ee") && strings.HasSuffix(secret, want) && len(secret) == 2+32+len(want)
}

// AdminSecretPath is Hiddify-style /{proxy_path_admin}/{admin_secret}
func AdminSecretPath() string {
	return SecretSegment(12) + "/" + SecretSegment(16)
}

func RandomPassword(n int) string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := RandomBytes(n)
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

func EnsureSelfSigned(certPath, keyPath, host string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil
		}
	}
	if host == "" {
		host = "localhost"
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host, Organization: []string{"soooski"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{host, "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		tmpl.DNSNames = []string{"localhost"}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	return nil
}

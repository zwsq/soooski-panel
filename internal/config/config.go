package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir     string
	ListenHTTP  string
	ListenHTTPS string
	PublicHost  string
	ACMEEmail   string
	SingBoxBin  string
	MtgBin      string
	AdminUser   string
	AdminPass   string
	CoreMode    string
	LogLevel    string
}

func Load() Config {
	c := Config{
		DataDir:     env("SOOOSKI_DATA_DIR", "/data"),
		ListenHTTP:  env("SOOOSKI_LISTEN_HTTP", ":80"),
		ListenHTTPS: env("SOOOSKI_LISTEN_HTTPS", ":443"),
		PublicHost:  env("SOOOSKI_PUBLIC_HOST", ""),
		ACMEEmail:   env("SOOOSKI_ACME_EMAIL", ""),
		SingBoxBin:  env("SOOOSKI_SINGBOX_BIN", "sing-box"),
		MtgBin:      env("SOOOSKI_MTG_BIN", "mtg-multi"),
		AdminUser:   env("SOOOSKI_ADMIN_USER", "admin"),
		AdminPass:   os.Getenv("SOOOSKI_ADMIN_PASSWORD"),
		CoreMode:    env("SOOOSKI_CORE_MODE", "embed"),
		LogLevel:    env("SOOOSKI_LOG_LEVEL", "info"),
	}
	c.DataDir = filepath.Clean(c.DataDir)
	c.CoreMode = strings.ToLower(c.CoreMode)
	return c
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func (c Config) DBPath() string      { return filepath.Join(c.DataDir, "soooski.db") }
func (c Config) CertDir() string     { return filepath.Join(c.DataDir, "certs") }
func (c Config) TLSCertPath() string { return filepath.Join(c.CertDir(), "server.crt") }
func (c Config) TLSKeyPath() string  { return filepath.Join(c.CertDir(), "server.key") }
func (c Config) ACMECacheDir() string {
	return filepath.Join(c.CertDir(), "acme")
}
func (c Config) SingBoxConfigPath() string {
	return filepath.Join(c.DataDir, "sing-box.json")
}
func (c Config) MtgConfigPath() string {
	return filepath.Join(c.DataDir, "mtg.toml")
}

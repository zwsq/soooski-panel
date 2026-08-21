package store

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/core/catalog"
	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(cfg config.Config) (*Store, string, error) {
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(cfg.CertDir(), 0755); err != nil {
		return nil, "", err
	}
	db, err := sql.Open("sqlite", cfg.DBPath()+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, "", err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, "", err
	}
	bootstrapPass, err := s.bootstrap(cfg)
	if err != nil {
		_ = db.Close()
		return nil, "", err
	}
	return s, bootstrapPass, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS admins (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  uuid TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  ss_password TEXT NOT NULL,
  sub_token TEXT NOT NULL UNIQUE,
  enable INTEGER NOT NULL DEFAULT 1,
  expire_at TEXT,
  traffic_limit INTEGER NOT NULL DEFAULT 0,
  traffic_up INTEGER NOT NULL DEFAULT 0,
  traffic_down INTEGER NOT NULL DEFAULT 0,
  wg_private_key TEXT NOT NULL DEFAULT '',
  wg_public_key TEXT NOT NULL DEFAULT '',
  wg_ip TEXT NOT NULL DEFAULT '',
  note TEXT NOT NULL DEFAULT '',
  telegram_secret TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS domains (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  domain TEXT NOT NULL UNIQUE,
  mode TEXT NOT NULL,
  enable INTEGER NOT NULL DEFAULT 1,
  provider TEXT NOT NULL DEFAULT 'none'
);
CREATE TABLE IF NOT EXISTS inbounds (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tag TEXT NOT NULL UNIQUE,
  protocol TEXT NOT NULL,
  transport TEXT NOT NULL,
  security TEXT NOT NULL,
  mode TEXT NOT NULL,
  listen_port INTEGER NOT NULL,
  internal_port INTEGER NOT NULL,
  path TEXT NOT NULL DEFAULT '',
  enable INTEGER NOT NULL DEFAULT 1,
  remark TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS sessions (
  token TEXT PRIMARY KEY,
  admin_id INTEGER NOT NULL,
  expires_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	return s.ensureUserColumn("telegram_secret", "TEXT NOT NULL DEFAULT ''")
}

func (s *Store) ensureUserColumn(name, decl string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('users') WHERE name=?`, name).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := s.db.Exec(`ALTER TABLE users ADD COLUMN ` + name + ` ` + decl)
	return err
}

func (s *Store) bootstrap(cfg config.Config) (string, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&n); err != nil {
		return "", err
	}
	printed := ""
	if n == 0 {
		pass := cfg.AdminPass
		if pass == "" {
			pass = crypto.RandomPassword(16)
			printed = pass
		}
		hash, err := crypto.HashPassword(pass)
		if err != nil {
			return "", err
		}
		if _, err := s.db.Exec(`INSERT INTO admins (username, password_hash) VALUES (?, ?)`, cfg.AdminUser, hash); err != nil {
			return "", err
		}
	}
	// Existing installs keep the dashboard username/password. Env
	// SOOOSKI_ADMIN_* is first-boot only so a compose default cannot lock
	// the account.

	priv, pub, err := crypto.RealityKeyPair()
	if err != nil {
		return "", err
	}
	ss := crypto.SS2022Password()
	wgPriv, wgPub, err := crypto.WireGuardKeyPair()
	if err != nil {
		return "", err
	}
	defaults := map[string]string{
		"public_host":          cfg.PublicHost,
		"reality_private":      priv,
		"reality_public":       pub,
		"reality_short_id":     crypto.ShortID(),
		"reality_server":       "www.microsoft.com",
		"ss_password":          ss,
		"wg_private":           wgPriv,
		"wg_public":            wgPub,
		"wg_subnet":            "10.66.66.0/24",
		"hy2_obfs":             "",
		"clash_secret":         crypto.RandomHex(16),
		"admin_path":           crypto.AdminSecretPath(),
		"client_path":          crypto.SecretSegment(12),
		"camouflage_url":       "",
		"tls_cert_path":        cfg.TLSCertPath(),
		"tls_key_path":         cfg.TLSKeyPath(),
		"acme_email":           cfg.ACMEEmail,
		"telegram_enabled":     "0",
		"telegram_fake_domain": "www.cloudflare.com",
		"telegram_secret":      "",
	}
	for k, v := range defaults {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO settings(key,value) VALUES(?,?)`, k, v); err != nil {
			return "", err
		}
	}
	if cfg.PublicHost != "" {
		_, _ = s.db.Exec(`UPDATE settings SET value=? WHERE key='public_host' AND value=''`, cfg.PublicHost)
	}
	if cfg.ACMEEmail != "" {
		_, _ = s.db.Exec(`UPDATE settings SET value=? WHERE key='acme_email' AND value=''`, cfg.ACMEEmail)
	}
	_, _ = s.db.Exec(`UPDATE settings SET value=? WHERE key='tls_cert_path'`, cfg.TLSCertPath())
	_, _ = s.db.Exec(`UPDATE settings SET value=? WHERE key='tls_key_path'`, cfg.TLSKeyPath())

	// Hiddify-style secret paths: never serve the panel at a public /panel or :8080.
	if v, _ := s.Setting("admin_path"); v == "" || v == "/panel" || v == "panel" {
		_ = s.SetSetting("admin_path", crypto.AdminSecretPath())
	}
	if v, _ := s.Setting("client_path"); v == "" {
		_ = s.SetSetting("client_path", crypto.SecretSegment(12))
	}
	_, _ = s.db.Exec(`UPDATE inbounds SET internal_port=12001 WHERE tag='vless-reality' AND internal_port=0`)

	if err := s.ensureInbounds(); err != nil {
		return "", err
	}
	if err := s.EnsureTelegramSecrets(); err != nil {
		return "", err
	}
	host := cfg.PublicHost
	if host == "" {
		host = "localhost"
	}
	if err := crypto.EnsureSelfSigned(cfg.TLSCertPath(), cfg.TLSKeyPath(), host); err != nil {
		return "", fmt.Errorf("tls cert: %w", err)
	}
	return printed, nil
}

func (s *Store) ensureInbounds() error {
	for _, spec := range catalog.Defaults() {
		path := spec.Path
		if spec.NeedsRandomPath {
			path = "/" + crypto.RandomPassword(10)
		}
		if _, err := s.db.Exec(`
INSERT OR IGNORE INTO inbounds(tag,protocol,transport,security,mode,listen_port,internal_port,path,enable,remark)
VALUES(?,?,?,?,?,?,?,?,1,?)`, spec.Tag, spec.Protocol, spec.Transport, spec.Security, spec.Mode,
			spec.ListenPort, spec.InternalPort, path, spec.Remark); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Setting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	return v, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) Settings() (models.Settings, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return models.Settings{}, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return models.Settings{}, err
		}
		m[k] = v
	}
	return models.Settings{
		PublicHost:         m["public_host"],
		RealityPrivateKey:  m["reality_private"],
		RealityPublicKey:   m["reality_public"],
		RealityShortID:     m["reality_short_id"],
		RealityServerName:  m["reality_server"],
		SSPassword:         m["ss_password"],
		WGPrivateKey:       m["wg_private"],
		WGPublicKey:        m["wg_public"],
		WGSubnet:           m["wg_subnet"],
		HY2Obfs:            m["hy2_obfs"],
		ClashSecret:        m["clash_secret"],
		CamouflageURL:      m["camouflage_url"],
		TLSCertPath:        m["tls_cert_path"],
		TLSKeyPath:         m["tls_key_path"],
		ACMEEmail:          m["acme_email"],
		AdminPath:          strings.Trim(m["admin_path"], "/"),
		ClientPath:         strings.Trim(m["client_path"], "/"),
		TelegramEnabled:    m["telegram_enabled"] == "1" || strings.EqualFold(m["telegram_enabled"], "true"),
		TelegramFakeDomain: m["telegram_fake_domain"],
		TelegramSecret:     m["telegram_secret"],
	}, rows.Err()
}

func (s *Store) AdminByUsername(username string) (models.Admin, error) {
	var a models.Admin
	err := s.db.QueryRow(`SELECT id, username, password_hash FROM admins WHERE username=?`, username).
		Scan(&a.ID, &a.Username, &a.PasswordHash)
	return a, err
}

func (s *Store) UpdateAdmin(id int64, username, passwordHash string) error {
	username = strings.TrimSpace(username)
	if username == "" && passwordHash == "" {
		return fmt.Errorf("nothing to update")
	}
	if username != "" && passwordHash != "" {
		_, err := s.db.Exec(`UPDATE admins SET username=?, password_hash=? WHERE id=?`, username, passwordHash, id)
		return err
	}
	if username != "" {
		_, err := s.db.Exec(`UPDATE admins SET username=? WHERE id=?`, username, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE admins SET password_hash=? WHERE id=?`, passwordHash, id)
	return err
}

func (s *Store) CreateSession(adminID int64, ttl time.Duration) (string, error) {
	token := crypto.RandomToken()
	exp := time.Now().Add(ttl).UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO sessions(token, admin_id, expires_at) VALUES(?,?,?)`, token, adminID, exp)
	return token, err
}

func (s *Store) SessionAdmin(token string) (models.Admin, error) {
	var a models.Admin
	var exp string
	err := s.db.QueryRow(`
SELECT a.id, a.username, a.password_hash, s.expires_at
FROM sessions s JOIN admins a ON a.id=s.admin_id WHERE s.token=?`, token).
		Scan(&a.ID, &a.Username, &a.PasswordHash, &exp)
	if err != nil {
		return a, err
	}
	t, err := time.Parse(time.RFC3339, exp)
	if err != nil || time.Now().After(t) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
		return models.Admin{}, sql.ErrNoRows
	}
	return a, nil
}

func (s *Store) DeleteSession(token string) {
	_, _ = s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
}

const userCols = `id,username,uuid,password,ss_password,sub_token,enable,expire_at,traffic_limit,traffic_up,traffic_down,wg_private_key,wg_public_key,wg_ip,note,telegram_secret,created_at`

func scanUser(sc interface{ Scan(dest ...any) error }) (models.User, error) {
	var u models.User
	var expire sql.NullString
	var enable int
	var created string
	err := sc.Scan(&u.ID, &u.Username, &u.UUID, &u.Password, &u.SSPassword, &u.SubToken,
		&enable, &expire, &u.TrafficLimit, &u.TrafficUp, &u.TrafficDown,
		&u.WGPrivateKey, &u.WGPublicKey, &u.WGIP, &u.Note, &u.TelegramSecret, &created)
	if err != nil {
		return u, err
	}
	u.Enable = enable != 0
	if expire.Valid && expire.String != "" {
		t, err := time.Parse(time.RFC3339, expire.String)
		if err == nil {
			u.ExpireAt = &t
		}
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return u, nil
}

func (s *Store) Users() ([]models.User, error) {
	rows, err := s.db.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []models.User{}
	}
	return out, rows.Err()
}

func (s *Store) UserByID(id int64) (models.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id=?`, id)
	return scanUser(row)
}

func (s *Store) UserByToken(token string) (models.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE sub_token=?`, token)
	return scanUser(row)
}

func nextWGIP(s *Store, subnet string) (string, error) {
	ip, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		ip, ipnet, err = net.ParseCIDR("10.66.66.0/24")
		if err != nil {
			return "", err
		}
	}
	used := map[string]bool{}
	rows, err := s.db.Query(`SELECT wg_ip FROM users`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return "", err
		}
		used[v] = true
	}
	ip = ip.To4()
	if ip == nil {
		return "", fmt.Errorf("wg subnet must be ipv4")
	}
	for i := 2; i < 254; i++ {
		cand := net.IPv4(ip[0], ip[1], ip[2], byte(i)).String()
		if !ipnet.Contains(net.ParseIP(cand)) {
			continue
		}
		if !used[cand] {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free wireguard ip in %s", subnet)
}

func (s *Store) CreateUser(username, note string, limit int64, expire *time.Time) (models.User, error) {
	settings, err := s.Settings()
	if err != nil {
		return models.User{}, err
	}
	wgPriv, wgPub, err := crypto.WireGuardKeyPair()
	if err != nil {
		return models.User{}, err
	}
	wgIP, err := nextWGIP(s, settings.WGSubnet)
	if err != nil {
		return models.User{}, err
	}
	domain := settings.TelegramFakeDomain
	if domain == "" {
		domain = "www.cloudflare.com"
	}
	u := models.User{
		Username:       username,
		UUID:           crypto.NewUUID(),
		Password:       crypto.RandomPassword(20),
		SSPassword:     crypto.SS2022Password(),
		SubToken:       crypto.RandomToken(),
		Enable:         true,
		ExpireAt:       expire,
		TrafficLimit:   limit,
		WGPrivateKey:   wgPriv,
		WGPublicKey:    wgPub,
		WGIP:           wgIP,
		Note:           note,
		TelegramSecret: crypto.TelegramFakeTLSSecret(domain),
		CreatedAt:      time.Now().UTC(),
	}
	var exp any
	if expire != nil {
		exp = expire.UTC().Format(time.RFC3339)
	}
	res, err := s.db.Exec(`
INSERT INTO users(username,uuid,password,ss_password,sub_token,enable,expire_at,traffic_limit,traffic_up,traffic_down,wg_private_key,wg_public_key,wg_ip,note,telegram_secret,created_at)
VALUES(?,?,?,?,?,1,?,?,0,0,?,?,?,?,?,?)`,
		u.Username, u.UUID, u.Password, u.SSPassword, u.SubToken, exp, u.TrafficLimit,
		u.WGPrivateKey, u.WGPublicKey, u.WGIP, u.Note, u.TelegramSecret, u.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return models.User{}, err
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

func (s *Store) UpdateUser(u models.User) error {
	var exp any
	if u.ExpireAt != nil {
		exp = u.ExpireAt.UTC().Format(time.RFC3339)
	}
	en := 0
	if u.Enable {
		en = 1
	}
	_, err := s.db.Exec(`UPDATE users SET username=?, enable=?, expire_at=?, traffic_limit=?, note=? WHERE id=?`,
		u.Username, en, exp, u.TrafficLimit, u.Note, u.ID)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) ResetTraffic(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET traffic_up=0, traffic_down=0 WHERE id=?`, id)
	return err
}

func (s *Store) AddTraffic(id int64, up, down int64) error {
	_, err := s.db.Exec(`UPDATE users SET traffic_up=traffic_up+?, traffic_down=traffic_down+? WHERE id=?`, up, down, id)
	return err
}

func (s *Store) DisableUser(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET enable=0 WHERE id=?`, id)
	return err
}

func (s *Store) SetTelegramSecret(id int64, secret string) error {
	_, err := s.db.Exec(`UPDATE users SET telegram_secret=? WHERE id=?`, secret, id)
	return err
}

func (s *Store) telegramDomain() string {
	st, err := s.Settings()
	if err != nil || strings.TrimSpace(st.TelegramFakeDomain) == "" {
		return "www.cloudflare.com"
	}
	return st.TelegramFakeDomain
}

func (s *Store) EnsureTelegramSecrets() error {
	domain := s.telegramDomain()
	users, err := s.Users()
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.TelegramSecret != "" && crypto.TelegramSecretMatchesDomain(u.TelegramSecret, domain) {
			continue
		}
		if u.TelegramSecret != "" {
			continue
		}
		if err := s.SetTelegramSecret(u.ID, crypto.TelegramFakeTLSSecret(domain)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RotateTelegramSecrets(domain string) error {
	if domain == "" {
		domain = s.telegramDomain()
	}
	users, err := s.Users()
	if err != nil {
		return err
	}
	for _, u := range users {
		if err := s.SetTelegramSecret(u.ID, crypto.TelegramFakeTLSSecret(domain)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Domains() ([]models.Domain, error) {
	rows, err := s.db.Query(`SELECT id, domain, mode, enable, provider FROM domains ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Domain
	for rows.Next() {
		var d models.Domain
		var en int
		if err := rows.Scan(&d.ID, &d.Domain, &d.Mode, &en, &d.Provider); err != nil {
			return nil, err
		}
		d.Enable = en != 0
		out = append(out, d)
	}
	if out == nil {
		out = []models.Domain{}
	}
	return out, rows.Err()
}

func (s *Store) CreateDomain(d models.Domain) (models.Domain, error) {
	d.Domain = strings.ToLower(strings.TrimSpace(d.Domain))
	en := 1
	if !d.Enable {
		en = 0
	}
	if d.Provider == "" {
		d.Provider = "none"
	}
	res, err := s.db.Exec(`INSERT INTO domains(domain,mode,enable,provider) VALUES(?,?,?,?)`, d.Domain, d.Mode, en, d.Provider)
	if err != nil {
		return d, err
	}
	d.ID, _ = res.LastInsertId()
	return d, nil
}

func (s *Store) UpdateDomain(d models.Domain) error {
	en := 0
	if d.Enable {
		en = 1
	}
	_, err := s.db.Exec(`UPDATE domains SET domain=?, mode=?, enable=?, provider=? WHERE id=?`,
		strings.ToLower(strings.TrimSpace(d.Domain)), d.Mode, en, d.Provider, d.ID)
	return err
}

func (s *Store) DeleteDomain(id int64) error {
	_, err := s.db.Exec(`DELETE FROM domains WHERE id=?`, id)
	return err
}

func (s *Store) Inbounds() ([]models.Inbound, error) {
	rows, err := s.db.Query(`SELECT id,tag,protocol,transport,security,mode,listen_port,internal_port,path,enable,remark FROM inbounds ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Inbound
	for rows.Next() {
		var in models.Inbound
		var en int
		if err := rows.Scan(&in.ID, &in.Tag, &in.Protocol, &in.Transport, &in.Security, &in.Mode,
			&in.ListenPort, &in.InternalPort, &in.Path, &en, &in.Remark); err != nil {
			return nil, err
		}
		in.Enable = en != 0
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *Store) SetInboundEnable(id int64, enable bool) error {
	en := 0
	if enable {
		en = 1
	}
	_, err := s.db.Exec(`UPDATE inbounds SET enable=? WHERE id=?`, en, id)
	return err
}

func (s *Store) Snapshot() (models.Settings, []models.User, []models.Domain, []models.Inbound, error) {
	st, err := s.Settings()
	if err != nil {
		return st, nil, nil, nil, err
	}
	users, err := s.Users()
	if err != nil {
		return st, nil, nil, nil, err
	}
	domains, err := s.Domains()
	if err != nil {
		return st, nil, nil, nil, err
	}
	inbounds, err := s.Inbounds()
	if err != nil {
		return st, nil, nil, nil, err
	}
	return st, users, domains, inbounds, nil
}

func DataFile(dir, name string) string {
	return filepath.Join(dir, name)
}

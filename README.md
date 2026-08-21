<p align="center">
  <img src="internal/web/dist/logo.svg" width="160" height="160" alt="soooski">
</p>

# soooski

A single-container multi-protocol proxy panel. Inspired by [Hiddify-Manager](https://github.com/hiddify/hiddify-manager), written in Go so it actually runs as a container: one process, SQLite on a volume, no systemd, no MariaDB, no HAProxy/Nginx/Xray soup.

The container **is** the panel and the server. Mount `/data` and keep it.

## What it does

- **Direct** — REALITY (Vision, gRPC, HTTP/2, xHTTP, HTTPUpgrade), Hysteria2, TUIC v5, ShadowTLS+SS2022, WireGuard, VLESS/VMess/Trojan TCP+TLS, and the same HTTP transports path-muxed on 443
- **CDN** — VLESS/VMess/Trojan over WebSocket, gRPC, HTTPUpgrade, HTTP/2, and xHTTP behind Cloudflare / Arvan / Gcore (plus Flexible HTTP on port 80)
- Optional **Telegram MTProto** FakeTLS — **per user** secret, traffic counted toward that user's quota
- **SNI mux on TCP 443** — your domain → HTTPS panel + path mux; Telegram fake domain → MTProto; other SNI → REALITY (same idea as Hiddify/HAProxy)
- **Let's Encrypt** — HTTP-01 on port 80, auto-renew, dashboard status; issued cert is used by the panel and by TLS inbounds
- **Secret admin and client paths** — not on a separate port; unknown URLs look like default nginx
- **User page** — open a subscription URL in a browser: Telegram proxy is the first card (QR + Open Telegram), then traffic, VPN subscription, and configs. Apps still get the raw sub.
- **Traffic accounting** — per-user via sing-box Clash API outbound chains, plus Telegram MTProto bytes on that user's secret
- **Admin account** — username, password, and the secret **admin path** are changeable in Settings; `soooski reset-admin` recovers a forgotten login (env vars are first boot only)

Protocol traffic is [sing-box](https://github.com/SagerNet/sing-box) 1.11.15 as a child process. Telegram FakeTLS is [mtg-multi](https://github.com/MHSanaei/mtg-multi) on localhost, one secret per user. The Go binary is the control plane, CDN path mux, admin UI, and camouflage site.

## Install

You need a Linux VPS. Ports **80** and **443** must be free. Do **not** compile on the server — GitHub Actions publishes `linux/amd64` and `linux/arm64` images to GHCR.

### Automatic

One line. Puts `soooski` in `/usr/local/bin` and opens a numbered menu. Choose **1** to install (Docker, image, domain, admin password). Later, run `sudo soooski` again for update, logs, URL, forgot-password, and the rest.

```bash
curl -fsSL https://raw.githubusercontent.com/zwsq/soooski-panel/release/install.sh | sudo bash
```

The menu reads the keyboard even when the script is piped (`/dev/tty`). Typical flow:

```
  1) Install panel
  2) Update panel
  3) Start
  4) Stop
  5) Restart
  6) Status
  7) Show admin URL
  8) View logs
  9) Reset admin username / password
 10) Reconfigure domain / Let's Encrypt email
 11) Uninstall
  0) Exit
```

Unattended (no menu) if you already know the flags:

```bash
curl -fsSL https://raw.githubusercontent.com/zwsq/soooski-panel/release/install.sh | sudo bash -s -- \
  --host vpn.example.com --email you@example.com --yes
```

After install, the same menu is just:

```bash
sudo soooski
```

Subcommands still work for scripts (`soooski url`, `soooski logs -f`, `soooski reset-admin -password '...'`).

`network_mode: host` is intentional: UDP 443, WireGuard, and extra REALITY ports bind on the VPS. Bookmark the admin URL.

### Manual

Same compose files the CLI uses. You still need Docker Compose.

```bash
# optional, only if the GitHub package is private:
# echo YOUR_GITHUB_PAT | docker login ghcr.io -u YOUR_GITHUB_USER --password-stdin

mkdir -p /opt/soooski
cd /opt/soooski
curl -fsSL -o docker-compose.yml https://raw.githubusercontent.com/zwsq/soooski-panel/release/docker-compose.yml
curl -fsSL -o .env.example https://raw.githubusercontent.com/zwsq/soooski-panel/release/.env.example
cp .env.example .env
# edit SOOOSKI_PUBLIC_HOST and SOOOSKI_ACME_EMAIL
nano .env

docker compose pull
docker compose up -d
docker compose logs -f
```

Or clone this repo and run `docker compose` from the root (same files).

Upgrade later: `sudo soooski` → **2) Update**, or `docker compose pull && docker compose up -d`.

`/opt/soooski/data` (or `./data`) is kept. New protocol tags are inserted on boot; existing users and settings are not wiped.

If `docker pull` says denied, open the GHCR package → **Package settings** → Public, or log in with a `read:packages` token.

Forgot the admin password: `sudo soooski` → **9**, or without the host CLI:

```bash
docker exec soooski soooski reset-admin
docker exec soooski soooski reset-admin -user admin -password 'your-new-pass'
```

## First boot

1. Set `SOOOSKI_PUBLIC_HOST` to a real hostname pointing at this VPS (A record). Placeholders like `vpn.example.com` are not sent to Let's Encrypt.
2. Set `SOOOSKI_ACME_EMAIL` so Let's Encrypt can reach you.
3. Optional: `SOOOSKI_ADMIN_PASSWORD`. Empty means a random password is printed **once** (install output or logs).
4. Username defaults to `SOOOSKI_ADMIN_USER` (`admin`). Change username, password, and the secret admin path later in Settings, or `soooski reset-admin` if you are locked out.
5. Point Cloudflare (if you use it) at origin **443**. Use **Full** until the dashboard shows a Let's Encrypt cert as **issued**, then **Full (strict)** is OK.

Those admin env vars apply **only on first boot**. After that the hash in SQLite wins, so a compose default cannot lock you out. Use `soooski reset-admin` (or `docker exec soooski soooski reset-admin`) if you forget them.

## Ports

With host networking these are the host ports.

| Port | What |
| --- | --- |
| 80 TCP | HTTP: secret paths, ACME HTTP-01, CDN Flexible, camouflage |
| 443 TCP | SNI mux: your domain → HTTPS panel + WS/gRPC/H2/xHTTP paths; other SNI → REALITY |
| 443 UDP | Hysteria2 |
| 10443 TCP | VLESS REALITY gRPC |
| 10444 TCP | VLESS REALITY HTTP/2 |
| 10445 TCP | VLESS REALITY HTTPUpgrade |
| 10446 TCP | VLESS REALITY xHTTP (HTTP/2 stand-in) |
| 8444 TCP | ShadowTLS + Shadowsocks 2022 |
| 8445 TCP | Trojan TCP + TLS |
| 8446 TCP | VMess TCP + TLS |
| 8447 TCP | VLESS TCP + TLS |
| 8448 UDP | TUIC v5 |
| 51820 UDP | WireGuard |

Enable or disable each inbound in the panel. Path-based protocols exist twice (direct hostname vs CDN hostname) so you can turn them independently.

## Direct vs CDN

Add domains in the panel.

- **direct** — A record to this machine. Used for REALITY / Hysteria2 / TUIC / ShadowTLS / WireGuard / raw TLS, and for path-muxed WS/gRPC/H2/HTTPUpgrade/xHTTP with your origin SNI on 443. REALITY handshake dest defaults to `www.microsoft.com`. Dest lookups are **IPv4-only** so a VPS without working IPv6 does not fail cloning that site.
- **cdn** — orange-cloud (or Arvan/Gcore). Origin **443** (Full TLS) or **80** (Flexible). Share links use the CDN hostname on 443. WS/HTTPUpgrade pin `alpn=http/1.1` so v2rayNG does not try HTTP/2 and fail the upgrade. gRPC and H2 use `h2`.

**HTTP/2 and xHTTP:** Hiddify `h2` and `xttp`/`xHTTP` both use sing-box 1.11 `http` transport. Share links are `type=http`. Native Xray split-HTTP is not in this core. SSH is not included.

Internet scanners hitting 443 with random SNI are forwarded to REALITY. That is expected.

## Panel HTTPS

| ClientHello SNI | Where it goes |
| --- | --- |
| `public_host` or a domain in the panel | HTTPS: admin UI, client subs, path-muxed proxies |
| An IP, or empty | HTTPS panel (`https://IP/`) |
| Telegram fake domain (default `www.cloudflare.com`) when Telegram is enabled | MTProto (mtg on `127.0.0.1:1001`) |
| Anything else (e.g. `www.microsoft.com`) | REALITY |

Certificates:

1. **Self-signed** in `/data/certs/` on first boot so HTTPS works immediately.
2. **Let's Encrypt** for `public_host` and every enabled domain. HTTP-01 on port 80; auto-renew ~30 days before expiry. Dashboard → **Issue / renew now**. Issued files are `/data/certs/server.crt` + `server.key` for the panel and for TLS inbounds.

Port 80 must be reachable from the internet for issuance.

## Subscription

Each user has `/{client_path}/{token}`:

- **Browser** → usage, expiry, QR, copyable configs
- v2rayNG / default app → base64 URI list
- Clash Meta / Stash → YAML
- sing-box / Hiddify app → JSON

Also `/{client_path}/{token}/clash`, `/sing-box`, `/v2ray`.

Traffic limits in the panel are entered in **GB / MB / TB** (or unlimited). Expiry is a calendar in the add/edit user modal (end of that UTC day). Status is `on`, `quota`, `expired`, or `off`. Hitting a quota removes the user from the proxy without locking the account: raising the limit turns them back on.

## Telegram proxy

Settings → **Telegram proxy** turns on FakeTLS MTProto on TCP 443 (SNI = the fake website, default `www.cloudflare.com`). Unlike Hiddify's shared proxy, **each user has a unique secret**. Their `tg://proxy` / `t.me/proxy` links are on that user's page. Bytes on that secret are polled from mtg-multi and **added to the same traffic_up / traffic_down as VPN**, so they count against the user's quota. Over-quota, expired, or disabled users are removed from the live secret set and cannot connect.

Do **not** set the fake domain to your panel hostname or the REALITY handshake dest. Changing the fake domain (or Settings → regenerate all) issues new secrets for every user.

`SOOOSKI_CORE_MODE=noop` skips both sing-box and mtg-multi.

## Env

| Variable | Default | When it applies |
| --- | --- | --- |
| `SOOOSKI_PUBLIC_HOST` | empty | always (share links + ACME) |
| `SOOOSKI_ACME_EMAIL` | empty | ACME contact |
| `SOOOSKI_ADMIN_USER` | `admin` | **first boot only** (change later in Settings or `soooski reset-admin`) |
| `SOOOSKI_ADMIN_PASSWORD` | random, printed once | **first boot only** (change later in Settings or `soooski reset-admin`) |
| `SOOOSKI_DATA` | `./data` | compose bind mount |
| `SOOOSKI_IMAGE` | branch tag / `:release` | which image to pull |
| `SOOOSKI_DATA_DIR` | `/data` | path inside the container |
| `SOOOSKI_LISTEN_HTTP` | `:80` | |
| `SOOOSKI_LISTEN_HTTPS` | `:443` | |
| `TZ` | `UTC` | |

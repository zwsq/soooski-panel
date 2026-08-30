# Changelog

All notable changes to soooski are documented in this file.

Version numbers are assigned when commits land on the `release` branch.
The first release is 0.1.0.

## [Unreleased]

### Bug Fixes
- Fix admin navigation, date picker, unlimited crash, and client order
- Fix client page Telegram placement, Open Telegram, and traffic bar
- Stop using `www.microsoft.com` as the REALITY handshake dest (certificate record overflow)
- Stop sending unknown REALITY ClientHellos to panel TLS (`tls: bad certificate`)
- Make admin and client usage bars readable on the dark theme
- Omit Xray-incompatible inbounds from the v2rayN / v2rayNG subscription (REALITY+HTTPUpgrade, sing-box HTTP/2 and xHTTP, ShadowTLS `ss://`)
- Emit a ShadowTLS v3 + SS2022 chain in the sing-box client JSON (plain `ss://` cannot handshake ShadowTLS)
- Apply TCP_NODELAY and half-close on the inner TCP connection when SNI-muxing REALITY

### Changes
- unlink from the fork network and recreate the repository
- Use the SVG Repo Telegram mark on the client page
- Users page: remaining-days counter only; orange when fewer than 10 days left

### Features
- Add host CLI, one-line install, and reset-admin
- Make the host CLI a numbered interactive menu
- Rebuild admin UI with React/shadcn and put Telegram first on the client page

### Miscellaneous
- chore:‌ swap logo
- Remove unused Vite scaffold asset


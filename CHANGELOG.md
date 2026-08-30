# Changelog

All notable changes to soooski are documented in this file.

Version numbers are assigned when commits land on the `release` branch.
The first release is 0.1.0.

## [0.1.1] - 2026-08-30

### Bug Fixes
- Fix REALITY behind the SNI mux and tighten the admin users page
- Fix REALITY dest overflow and dark-theme usage bars

### Changes
- Send unknown 443 SNI back to REALITY, not panel TLS
- Filter Xray-incompatible v2rayN links and simplify expire UI

### Features
- Add mobile admin layout and last-seen / online status

## [0.1.0] - 2026-08-21

### Bug Fixes
- Fix admin navigation, date picker, unlimited crash, and client order
- Fix client page Telegram placement, Open Telegram, and traffic bar

### Changes
- unlink from the fork network and recreate the repository
- Use the SVG Repo Telegram mark on the client page

### Features
- Add host CLI, one-line install, and reset-admin
- Make the host CLI a numbered interactive menu
- Rebuild admin UI with React/shadcn and put Telegram first on the client page
- Add changelog generation and semver tags on release

### Miscellaneous
- chore:‌ swap logo
- Remove unused Vite scaffold asset


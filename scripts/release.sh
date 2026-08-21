#!/usr/bin/env bash
# Cut a semver release from git history (git-cliff).
#
# Used by .github/workflows/release.yml when commits land on `release`.
#   scripts/release.sh --dry-run   # print next version + notes
#   scripts/release.sh --commit    # write VERSION + CHANGELOG, commit, tag
#   scripts/release.sh --push      # --commit, then push branch + tag
set -euo pipefail

usage() {
  cat <<'EOF'
Cut a semver release from git history (git-cliff).

  scripts/release.sh --dry-run   print next version + notes
  scripts/release.sh --commit    write VERSION + CHANGELOG, commit, tag
  scripts/release.sh --push      --commit, then push branch + tag
EOF
}

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

MODE=dry-run
for arg in "$@"; do
  case "$arg" in
    --dry-run) MODE=dry-run ;;
    --commit) MODE=commit ;;
    --push) MODE=push ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown flag: $arg" >&2; usage >&2; exit 2 ;;
  esac
done

if ! command -v git-cliff >/dev/null 2>&1; then
  echo "git-cliff is required (https://git-cliff.org)" >&2
  exit 1
fi

if [[ -z "$(git tag -l 'v[0-9]*')" ]]; then
  echo "no v* tags yet; first release is 0.1.0"
fi

if git describe --exact-match --match 'v[0-9]*' HEAD >/dev/null 2>&1; then
  echo "HEAD is already tagged $(git describe --exact-match --match 'v[0-9]*' HEAD); nothing to do"
  exit 0
fi

msg="$(git log -1 --pretty=%s)"
if [[ "$msg" == chore\(release\):* ]]; then
  echo "HEAD is a release commit; skip"
  exit 0
fi

NEXT_RAW="$(git-cliff --bumped-version | tr -d '[:space:]')"
NEXT="${NEXT_RAW#v}"
if [[ ! "$NEXT" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "git-cliff --bumped-version produced ${NEXT_RAW@Q}" >&2
  exit 1
fi

LAST="$(git tag -l 'v[0-9]*' --sort=-v:refname | head -n1 || true)"
if [[ -n "$LAST" ]]; then
  count="$(git rev-list --count "${LAST}..HEAD")"
  if [[ "$count" -eq 0 ]]; then
    echo "no commits since $LAST"
    exit 0
  fi
  if [[ "$LAST" == "v$NEXT" ]]; then
    echo "next version still $LAST; nothing to bump"
    exit 0
  fi
fi

echo "next version: $NEXT"
git-cliff --bump --unreleased --strip all

if [[ "$MODE" == "dry-run" ]]; then
  exit 0
fi

echo "$NEXT" > VERSION
git-cliff --bump -o CHANGELOG.md

git add VERSION CHANGELOG.md
if git diff --cached --quiet; then
  echo "VERSION and CHANGELOG.md already match $NEXT"
else
  git commit -m "chore(release): v${NEXT}"
fi

if git rev-parse "v${NEXT}" >/dev/null 2>&1; then
  echo "tag v${NEXT} already exists"
else
  git tag -a "v${NEXT}" -m "v${NEXT}"
fi

if [[ "$MODE" != "push" ]]; then
  exit 0
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
git push origin "HEAD:refs/heads/${branch}"
git push origin "v${NEXT}"

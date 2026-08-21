.PHONY: build test docker web

VERSION ?= $(shell tr -d '[:space:]' < VERSION 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/zwsq/soooski-panel/internal/version.Version=$(VERSION)

web:
	cd web && npm ci && npm run build

build:
	go build -ldflags="$(LDFLAGS)" -o soooski ./cmd/soooski

test:
	go test ./...
	bash -n install.sh scripts/soooski scripts/release.sh
	grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' VERSION
	./scripts/soooski help >/dev/null

# local debug image only — VPS should pull ghcr.io/zwsq/soooski-panel
docker:
	docker build --build-arg VERSION=$(VERSION) -t soooski:local .

.PHONY: build test docker

build:
	go build -o soooski ./cmd/soooski

test:
	go test ./...
	bash -n install.sh scripts/soooski
	./scripts/soooski help >/dev/null

# local debug image only — VPS should pull ghcr.io/zwsq/soooski-panel
docker:
	docker build -t soooski:local .

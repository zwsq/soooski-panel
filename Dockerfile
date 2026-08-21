FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/soooski ./cmd/soooski

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget tar
ARG TARGETARCH
ARG SINGBOX_VERSION=1.11.15
ARG MTG_VERSION=1.15.0
RUN arch="$TARGETARCH"; \
    if [ -z "$arch" ]; then arch="$(uname -m)"; fi; \
    case "$arch" in \
      amd64|x86_64) arch=amd64 ;; \
      arm64|aarch64) arch=arm64 ;; \
      *) echo "unsupported arch: $arch" && exit 1 ;; \
    esac; \
    wget -qO /tmp/sb.tgz "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${arch}.tar.gz"; \
    tar -xzf /tmp/sb.tgz -C /tmp; \
    mv /tmp/sing-box-*/sing-box /usr/local/bin/sing-box; \
    chmod +x /usr/local/bin/sing-box; \
    rm -rf /tmp/sb.tgz /tmp/sing-box-*; \
    wget -qO /tmp/mtg.tgz "https://github.com/MHSanaei/mtg-multi/releases/download/v${MTG_VERSION}/mtg-multi-${MTG_VERSION}-linux-${arch}.tar.gz"; \
    tar -xzf /tmp/mtg.tgz -C /tmp; \
    bin="$(find /tmp -maxdepth 3 -type f -name mtg-multi | head -n1)"; \
    mv "$bin" /usr/local/bin/mtg-multi; \
    chmod +x /usr/local/bin/mtg-multi; \
    rm -rf /tmp/mtg.tgz /tmp/mtg-multi*
COPY --from=build /out/soooski /usr/local/bin/soooski
ENV SOOOSKI_DATA_DIR=/data \
    SOOOSKI_SINGBOX_BIN=/usr/local/bin/sing-box \
    SOOOSKI_MTG_BIN=/usr/local/bin/mtg-multi \
    TZ=UTC
VOLUME ["/data"]
EXPOSE 80/tcp 443/tcp 443/udp 8444/tcp 8445/tcp 8446/tcp 8447/tcp 8448/udp 10443/tcp 10444/tcp 10445/tcp 10446/tcp 51820/udp
ENTRYPOINT ["/usr/local/bin/soooski"]

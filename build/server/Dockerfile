# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

FROM --platform=$BUILDPLATFORM docker.io/alpine:3.22.1 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APPVERSION

WORKDIR /build

# Copy go from official image.
COPY --from=docker.io/golang:1.25.1-alpine /usr/local/go/ /usr/local/go/
# Copy bun from official image
COPY --from=docker.io/oven/bun:1.2.23-alpine /usr/local/bin/bun /usr/local/bin/bun
# Update $PATH.
ENV PATH="/root/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

# Install tools.
RUN apk add libstdc++ upx

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN mkdir -p pkg/go-syndication pkg/slog-elasticsearch
COPY pkg/go-syndication/go.mod pkg/go-syndication/go.sum ./pkg/go-syndication
COPY pkg/slog-elasticsearch/go.mod pkg/slog-elasticsearch/go.sum ./pkg/slog-elasticsearch
RUN go mod download

# Copy source.
COPY . .

# install and build/bundle frontend assets
RUN <<EOF
bun install
bun x esbuild ./web/assets/scripts.js --bundle --minify --outdir=./web/content/
bun x tailwindcss -i ./web/assets/styles.css -o ./web/content/styles.css --minify
EOF

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X github.com/immanent-tech/foragd/config.Version=$APPVERSION" -o foragd

# compress binary with upx
RUN upx --best --lzma foragd

FROM docker.io/alpine AS server

# Add labels.
LABEL org.opencontainers.image.source=https://github.com/immanent-tech/foragd
LABEL org.opencontainers.image.url=https://foragd.app
LABEL org.opencontainers.image.title="Foragd Server"
LABEL org.opencontainers.image.description="Server service for Foragd app is responsible for handling all web requests."
LABEL org.opencontainers.image.licenses=AGPL-3.0-or-later

# Install CA certificates.
RUN apk add ca-certificates

# Copy project's binary and templates from /build to the scratch container.
COPY --from=builder /build/foragd /

# Set entry point.
ENTRYPOINT ["/foragd"]
CMD ["serve", "--no-log-file"]

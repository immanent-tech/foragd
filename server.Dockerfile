# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

# Alpine base.
# https://hub.docker.com/_/alpine/
FROM --platform=$BUILDPLATFORM docker.io/alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APPVERSION

WORKDIR /build

# Copy go from official image.
# https://hub.docker.com/_/golang
COPY --from=docker.io/golang:1.26.3-alpine3.23@sha256:f44b851aa23dfa219d18db6eab743203245429d355cb619cf96a2ffe2a84ba7a /usr/local/go/ /usr/local/go/
# Update $PATH.
ENV PATH="/root/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

# Install tools.
RUN apk add libstdc++ upx npm

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN mkdir -p pkg/go-syndication pkg/slog-elasticsearch pkg/slog-chi
COPY pkg/go-syndication/go.mod pkg/go-syndication/go.sum ./pkg/go-syndication/
COPY pkg/slog-elasticsearch/go.mod pkg/slog-elasticsearch/go.sum ./pkg/slog-elasticsearch/
COPY pkg/slog-chi/go.mod pkg/slog-chi/go.sum ./pkg/slog-chi/
RUN go mod download

# Copy source.
COPY . .

# install and build/bundle frontend assets
RUN <<EOF
npm clean-install && \
    npm run build:prod && \
    npm version patch
EOF

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X github.com/immanent-tech/foragd/config.Version=$APPVERSION" -o foragd

# compress binary with upx
RUN upx --best --lzma foragd

FROM docker.io/alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS server

ENV FORAGD_CONTAINER=1

# Add labels.
LABEL org.opencontainers.image.source="https://github.com/immanent-tech/foragd"
LABEL org.opencontainers.image.url="https://foragd.app"
LABEL org.opencontainers.image.title="Foragd Server"
LABEL org.opencontainers.image.description="Server service for Foragd app is responsible for handling all web requests."
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"

# Install supporting packages required for certain functionality.
RUN apk add ca-certificates tzdata

# Copy project's binary and templates from /build to the scratch container.
COPY --from=builder /build/foragd /

# Allow custom uid and gid
ARG UID=1000
ARG GID=1000

# Add user
RUN addgroup --gid "${GID}" foragd && \
    adduser --disabled-password --gecos "" --ingroup foragd \
    --uid "${UID}" foragd
USER foragd

# Set entry point.
ENTRYPOINT ["/foragd"]
CMD ["serve", "--no-log-file"]

# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

# https://hub.docker.com/_/alpine/
ARG ALPINE_VERSION=3.24.1@sha256:79ff19e9084a00eece421b2523fb93e22d730e2c0e525905de047e848e56d95f
# https://hub.docker.com/_/golang
ARG GO_VERSION=1.26.6-alpine3.24@sha256:1a9c10cf505a9e6b1e96ea77ebdbfe79a0f10380181faf88bc3b51d7e4315fae

FROM --platform=$BUILDPLATFORM docker.io/golang:${GO_VERSION} AS golang
FROM --platform=$BUILDPLATFORM docker.io/alpine:${ALPINE_VERSION} AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APPVERSION

WORKDIR /build

# Copy go from official image.
COPY --from=golang /usr/local/go/ /usr/local/go/
# Update $PATH.
ENV PATH="/root/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

# Install tools.
RUN apk add libstdc++ upx npm

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN mkdir -p pkg/go-syndication
COPY pkg/go-syndication/go.mod pkg/go-syndication/go.sum ./pkg/go-syndication/
COPY base/go.mod base/go.sum ./base/
RUN go mod download

# Copy source.
COPY . .

# install and build/bundle frontend assets
RUN npm clean-install && \
    npx npm-run-all --parallel prod:css prod:js && \
    npm version patch

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o foragd

# compress binary with upx
RUN upx --best --lzma foragd

FROM docker.io/alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS server

ENV FORAGD_CONTAINER=1

# Add labels.
LABEL org.opencontainers.image.source="https://github.com/immanent-tech/foragd"
LABEL org.opencontainers.image.url="https://foragd.app"
LABEL org.opencontainers.image.title="Foragd Server"
LABEL org.opencontainers.image.description="Server service for Foragd app is responsible for handling all web requests."
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"

# Install supporting packages required for certain functionality.
RUN apk add ca-certificates tzdata

# Add the Zyte CA cert.
ADD https://docs.zyte.com/_static/zyte-ca.crt /usr/local/share/ca-certificates/zyte-ca.crt
RUN update-ca-certificates

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
CMD ["serve"]

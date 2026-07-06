# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

# https://hub.docker.com/_/alpine/
ARG ALPINE_VERSION=3.24.0@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4
# https://hub.docker.com/_/golang
ARG GO_VERSION=1.26.4-alpine3.24@sha256:7a3e50096189ad57c9f9f865e7e4aa8585ed1585248513dc5cda498e2f41812c

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
RUN go mod download

# Copy source.
COPY . .

# install and build/bundle frontend assets
RUN npm clean-install && \
    npm run build:prod && \
    npm version patch

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X github.com/immanent-tech/foragd/config.Version=$APPVERSION" -o foragd

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
CMD ["serve", "--no-log-file"]

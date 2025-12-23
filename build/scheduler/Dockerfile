# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

# Alpine base.
# https://hub.docker.com/_/alpine/
FROM --platform=$BUILDPLATFORM docker.io/alpine:3.23.2@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG APPVERSION

# Copy go from official image.
# https://hub.docker.com/_/golang
COPY --from=docker.io/golang:1.25.5-alpine@sha256:ac09a5f469f307e5da71e766b0bd59c9c49ea460a528cc3e6686513d64a6f1fb /usr/local/go/ /usr/local/go/
# Update $PATH.
ENV PATH="/root/go/bin:/usr/local/go/bin:/usr/local/bin:${PATH}"

# Install tools.
RUN apk add upx

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN mkdir -p pkg/go-syndication pkg/slog-elasticsearch
COPY pkg/go-syndication/go.mod pkg/go-syndication/go.sum ./pkg/go-syndication
COPY pkg/slog-elasticsearch/go.mod pkg/slog-elasticsearch/go.sum ./pkg/slog-elasticsearch
RUN go mod download

# Copy source.
COPY . .

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X github.com/immanent-tech/foragd/config.Version=$APPVERSION" -o foragd

# compress binary with upx
RUN upx --best --lzma foragd

FROM docker.io/alpine:3.23.2@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS scheduler

ENV FORAGD_CONTAINER=1

# Add labels.
LABEL org.opencontainers.image.source=https://github.com/immanent-tech/foragd
LABEL org.opencontainers.image.url=https://foragd.app
LABEL org.opencontainers.image.title="Foragd Scheduler"
LABEL org.opencontainers.image.description="Scheduler service for Foragd app is responsible for managing and executing background jobs."
LABEL org.opencontainers.image.licenses=AGPL-3.0-or-later

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
CMD ["scheduler", "--no-log-file", "run"]

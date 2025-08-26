# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

FROM golang:1.25.0-alpine AS builder

# Move to working directory (/build).
WORKDIR /build

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Copy your code into the container.
COPY . .

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w" -o go_feed_me

FROM scratch

# Copy project's binary and templates from /build to the scratch container.
COPY --from=builder /build/go_feed_me /

# Set entry point.
ENTRYPOINT ["/go_feed_me"]
CMD ["scheduler"]

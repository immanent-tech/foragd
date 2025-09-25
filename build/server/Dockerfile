# Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
# SPDX-License-Identifier: 	AGPL-3.0-or-later

ARG GOVERSION
FROM golang:$GOVERSION-alpine AS builder

ARG APPVERSION

# Move to working directory (/build).
WORKDIR /build

RUN apk add git


# Copy your code into the container.
COPY . .

# Copy and download dependency using go mod.
COPY go.mod go.sum ./
RUN go mod download

# Set necessary environment variables and build your project.
ENV CGO_ENABLED=0
RUN go build -ldflags="-s -w -X github.com/immanent-tech/foragd/config.Version=$APPVERSION" -o foragd

FROM scratch

# Copy project's binary and templates from /build to the scratch container.
COPY --from=builder /build/foragd /

# Set entry point.
ENTRYPOINT ["/foragd"]
CMD ["serve"]

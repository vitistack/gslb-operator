FROM golang:1.27-alpine AS build

LABEL MAINTAINER="espen.wobbes@nhn.no"

ARG VERSION
ARG DATE
# RACE=1/true builds with the race detector (CGO), anything else builds without it.
ARG RACE=false

WORKDIR /app

# build-base (gcc/musl-dev) is required when building with the race detector (CGO)
RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN set -eux; \
    if [ "$RACE" = "1" ] || [ "$RACE" = "true" ]; then \
        echo "Building WITH race detector"; \
        CGO_ENABLED=1 go build -race -ldflags "-X github.com/vitistack/gslb-operator/internal/config.Version=${VERSION:-} -X github.com/vitistack/gslb-operator/internal/config.BuildDate=${DATE:-}" -o gslb-operator ./cmd; \
    else \
        echo "Building WITHOUT race detector"; \
        CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/vitistack/gslb-operator/internal/config.Version=${VERSION:-} -X github.com/vitistack/gslb-operator/internal/config.BuildDate=${DATE:-}" -o gslb-operator ./cmd; \
    fi

FROM alpine:3.23

WORKDIR /app

# libgcc is required at runtime by race-enabled (CGO) builds
RUN apk add --no-cache libgcc

# create group and user that will own the application workspace
RUN addgroup -g 1000 -S gslb-group && adduser -u 1000 -S gslb-operator -G gslb-group

COPY --from=build /app/gslb-operator /app/gslb-operator
COPY sandbox.lua /app

# change ownership of directory
RUN chown -R gslb-operator:gslb-group /app

# sandbox is read-only
RUN chmod 440 sandbox.lua
USER gslb-operator

CMD [ "./gslb-operator" ]
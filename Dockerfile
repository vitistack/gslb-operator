FROM golang:1.27-alpine AS build

LABEL MAINTAINER="espen.wobbes@nhn.no"

ARG VERSION
ARG DATE
# Optional override: RACE=1 forces on, RACE=0 forces off. Empty = auto-detect from VERSION.
ARG RACE

WORKDIR /app

# build-base (gcc/musl-dev) is required when building with the race detector (CGO)
RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Auto-enable the race detector for staging/rc tags (e.g. v1.2.3-staging, v1.2.3-rc1).
RUN set -eux; \
    race="${RACE}"; \
    if [ -z "$race" ]; then \
        case "$VERSION" in \
            *staging*|*-rc*) race=1 ;; \
            *) race=0 ;; \
        esac; \
    fi; \
    if [ "$race" = "1" ] || [ "$race" = "true" ]; then \
        echo "Building WITH race detector (VERSION=${VERSION})"; \
        CGO_ENABLED=1 go build -race -ldflags "-X main.version=${VERSION} -X main.buildDate=${DATE}" -o gslb-operator ./cmd/main.go; \
    else \
        echo "Building WITHOUT race detector (VERSION=${VERSION})"; \
        CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.buildDate=${DATE}" -o gslb-operator ./cmd/main.go; \
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
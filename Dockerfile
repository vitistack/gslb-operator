FROM golang:1.26 AS build

LABEL MAINTAINER="espen.wobbes@nhn.no"

ARG VERSION
ARG DATE

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# build image
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.buildDate=${DATE}" -o gslb-operator ./cmd/main.go


FROM alpine:3.23

WORKDIR /app

RUN addgroup -S gslb-group && adduser -S gslb-operator -G gslb-group
RUN chown -R gslb-operator:gslb-group /app

COPY --from=build /app/gslb-operator /app/gslb-operator
COPY sandbox.lua /app

USER gslb-operator

CMD [ "./gslb-operator" ]

# syntax=docker/dockerfile:1
# Compilation croisée sur la machine de build (rapide pour arm64 / Raspberry Pi), image finale minimale.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/server ./cmd/server \
 && GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w" -o /out/import ./cmd/import

FROM alpine:3.20
RUN apk add --no-cache tzdata ca-certificates wget && adduser -D -u 10001 app
COPY --from=build /out/ /app/
USER app
ENV GIN_MODE=release PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=60s CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
ENTRYPOINT ["/app/server"]

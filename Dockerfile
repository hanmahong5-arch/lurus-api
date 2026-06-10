FROM oven/bun:latest AS builder

WORKDIR /build
COPY web/package.json .
COPY web/bun.lock .
RUN bun install
COPY ./web .
COPY ./VERSION .
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

FROM golang:alpine AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

# Copy proto-go module (identity gRPC contract types)
COPY lurus-proto-go/ /shared/lurus-proto-go/

# Copy zita-sdk-go (identity SDK, ADR-0011) — pinned via CI checkout ref
COPY zita-sdk-go/ /shared/zita-sdk-go/

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/LurusTech/lurus-hub/internal/pkg/common.Version=$(cat VERSION)'" -o lurus-api ./cmd/server

FROM debian:bookworm-slim

# apt-get upgrade applies the latest Debian security patches (e.g. libgnutls30
# 3.7.9-2+deb12u7, pulled in transitively by wget) — without it the GHA layer
# cache can pin an older, CVE-flagged package set. Keep this so the Trivy gate
# stays green on base-image CVEs.
RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/lurus-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/lurus-api"]

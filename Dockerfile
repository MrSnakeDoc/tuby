# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS=linux
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE

RUN build_date="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w \
      -X github.com/MrSnakeDoc/tuby/internal/version.Version=${VERSION} \
      -X github.com/MrSnakeDoc/tuby/internal/version.Commit=${GIT_COMMIT} \
      -X github.com/MrSnakeDoc/tuby/internal/version.BuildDate=${build_date}" \
    -o /out/tuby \
    ./cmd/tuby

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/tuby /app/tuby

VOLUME ["/app/data"]

ENTRYPOINT ["/app/tuby"]

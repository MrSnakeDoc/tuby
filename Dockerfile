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
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -buildmode=pie \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w \
      -X git.nexacloud.dev/MrSnakeDoc/tuby/internal/version.Version=${VERSION} \
      -X git.nexacloud.dev/MrSnakeDoc/tuby/internal/version.Commit=${GIT_COMMIT} \
      -X git.nexacloud.dev/MrSnakeDoc/tuby/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/tuby \
    ./cmd/tuby

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/tuby /app/tuby

VOLUME ["/app/data"]

ENTRYPOINT ["/app/tuby"]

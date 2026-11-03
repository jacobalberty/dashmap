# syntax=docker/dockerfile:1

# Stage 1: build the Vue SPA (platform-independent output, so always run natively)
FROM --platform=$BUILDPLATFORM node:24-trixie-slim AS frontend
WORKDIR /build
COPY internal/web/package*.json ./
RUN npm ci
COPY internal/web/ ./
RUN npm run build

# Stage 2: cross-compile the Go binary with the embedded SPA
FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./internal/web/dist
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -tags embedspa \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /dashmap ./cmd/dashmap

# Stage 3: minimal distroless runtime (non-root)
FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.title="dashmap" \
      org.opencontainers.image.description="Browse Tesla dashcam footage on a map and calendar" \
      org.opencontainers.image.source="https://github.com/jacobalberty/dashmap"
COPY --from=backend /dashmap /dashmap
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/dashmap", "healthcheck"]
ENTRYPOINT ["/dashmap"]
CMD ["serve"]

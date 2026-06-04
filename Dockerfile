# Stage 1: build the Vue SPA
FROM node:24-trixie-slim AS frontend
WORKDIR /build
COPY internal/web/package*.json ./
RUN npm ci
COPY internal/web/ ./
RUN npm run build

# Stage 2: build the Go binary with the embedded SPA
FROM golang:1.26-bookworm AS backend
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -tags embedspa -o /dashmap ./cmd/dashmap

# Stage 3: minimal distroless runtime
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=backend /dashmap /dashmap
ENTRYPOINT ["/dashmap"]
CMD ["serve"]

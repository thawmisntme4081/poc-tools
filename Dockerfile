# Dockerfile for StockMind Application
# Build: docker build -t stockmind-backend .
# Run: docker run -p 8080:8080 stockmind-backend

# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.25.1-alpine AS backend

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY schema/ ./schema/

RUN go build -o app ./cmd/main.go

FROM node:20-alpine AS frontend

WORKDIR /home/apps
COPY --chown=apps:apps ./frontend /home/apps

RUN npm ci && \
    npm run build

# =============================================================================
# Runtime Stage
# =============================================================================
FROM alpine:latest

# Cài curl để healthcheck
RUN apk add --no-cache curl ca-certificates

# Copy backend binary
COPY --from=backend /app/app /usr/local/bin/stockmind

# Copy migrations từ backend stage (COPY trước khi tạo user)
COPY --from=backend /app/schema/migrations /app/schema/migrations

# Copy frontend dist
COPY --from=frontend /home/apps/dist /app/frontend/dist

# Tạo user và set permissions cho migrations
RUN addgroup -g 1001 -S stockmind && \
    adduser -S stockmind -u 1001 -G stockmind && \
    chown -R stockmind:stockmind /app/schema/migrations && \
    chmod -R 755 /app/schema/migrations

WORKDIR /app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/v1/health || exit 1

# Chạy ứng dụng với user stockmind
USER stockmind

CMD ["/usr/local/bin/stockmind", "server", "--port", "8080"]
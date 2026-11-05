# Stage 1: Build Web Studio
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go Gateway
FROM golang:1.24-alpine AS go-builder
RUN apk add --no-cache build-base git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /imgscale ./cmd/imgscale

# Stage 3: Lightweight Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /imgscale /usr/local/bin/imgscale

EXPOSE 8080
VOLUME ["/data/images", "/data/cache"]

ENTRYPOINT ["/usr/local/bin/imgscale"]
CMD ["serve", "--port", "8080", "--storage-dir", "/data/images", "--cache-dir", "/data/cache"]

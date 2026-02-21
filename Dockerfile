# Stage 1: Build Frontend
FROM node:22-alpine AS ui-builder

WORKDIR /app/ui
RUN corepack enable && corepack prepare pnpm@latest --activate

COPY ui/package.json ui/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY ui/ .
RUN pnpm build

# Stage 2: Build Go Binary
FROM golang:1.24-alpine AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o log-sentry ./cmd/server

# Stage 3: Runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y systemd && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=go-builder /app/log-sentry .
COPY --from=ui-builder /app/ui/dist ./ui/dist

# Copy default config files
COPY services.json .
COPY rules.json* ./

CMD ["./log-sentry"]

FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o log-sentry ./cmd/server

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/log-sentry .

# Install necessary runtime deps if any (not strictly needed for static binary but good for debugging)
# RUN apk add --no-cache bash

CMD ["./log-sentry"]

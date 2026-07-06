FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /smsrelay ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache sqlite-libs ca-certificates wget
WORKDIR /app
COPY --from=builder /smsrelay /app/smsrelay
COPY db/migrations /app/db/migrations
RUN mkdir -p /data
ENV DATABASE_PATH=/data/smsrelay.db
ENV MIGRATIONS_DIR=/app/db/migrations
EXPOSE 8080
CMD ["/app/smsrelay"]

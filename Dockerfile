FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux go build -o mcp-calculator-server ./server.go

FROM alpine:3.21

RUN apk --no-cache add ca-certificates wget

RUN addgroup -g 1001 -S mcp && \
    adduser -S -D -H -u 1001 -s /sbin/nologin -G mcp mcp

WORKDIR /app

COPY --from=builder /app/mcp-calculator-server .

RUN chown mcp:mcp mcp-calculator-server

USER mcp

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./mcp-calculator-server"]
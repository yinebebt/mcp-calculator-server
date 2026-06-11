FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o mcp-calculator-server .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=builder /app/mcp-calculator-server .
EXPOSE 8080
CMD ["./mcp-calculator-server"]
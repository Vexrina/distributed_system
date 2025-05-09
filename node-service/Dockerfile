FROM golang:1.24.3-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o node-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/node-service .
EXPOSE 8080
CMD ["./node-service"] 
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Устанавливаем curl для health check
RUN apk add --no-cache curl ca-certificates

# Копируем go mod файлы
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /root/

# Копируем собранное приложение
COPY --from=builder /app/main .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]
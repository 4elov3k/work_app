FROM golang:1.25-alpine AS builder

WORKDIR /app
ARG APP=server

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN go build -o /app/app ./cmd/${APP}

# Финальный образ
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем бинарник из builder
COPY --from=builder /app/app .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./app"]

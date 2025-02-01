# Этап сборки
FROM golang:1.23.0-alpine AS builder

WORKDIR /build

# Копируем сначала только файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Финальный этап
FROM alpine:3.19

WORKDIR /app

# Копируем только бинарный файл из этапа сборки
COPY --from=builder /build/main .

# Создаем директорию для логов
RUN mkdir -p /app/logs

EXPOSE 8080

CMD ["./main"]
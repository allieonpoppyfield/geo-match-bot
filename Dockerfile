# Используем Go 1.21-alpine как базовый образ
FROM golang:1.23-alpine

# Устанавливаем рабочую директорию
WORKDIR /app

# Устанавливаем необходимые пакеты для сборки и работы с librdkafka
RUN apk add --no-cache \
    gcc \
    libc-dev \
    librdkafka-dev \
    pkgconfig \
    build-base \
    git

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект в контейнер
COPY . .

# Устанавливаем переменные окружения для сборки с поддержкой C-библиотек
ENV CGO_ENABLED=1

# Сборка приложения
RUN go build -tags dynamic -o geo_match_bot ./cmd

# Запуск приложения
CMD ["./geo_match_bot"]

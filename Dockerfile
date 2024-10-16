# Используем Go 1.22 как базовый образ
FROM golang:1.22-alpine

# Устанавливаем рабочую директорию
WORKDIR /app

# Устанавливаем необходимые зависимости
RUN apk --no-cache add git

# Копируем go.mod и go.sum для установки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект в контейнер
COPY . .

# Сборка приложения с использованием vendor
RUN go build -o geo_match_bot ./cmd

# Запуск приложения
CMD ["./geo_match_bot"]

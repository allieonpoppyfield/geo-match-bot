package main

import (
	"geo_match_bot/internal/bot"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/config"
	"geo_match_bot/internal/db"
	"geo_match_bot/internal/handlers"
	"geo_match_bot/internal/messaging"
	"geo_match_bot/internal/repository"
	"log"
)

func main() {
	// Загружаем конфигурацию
	cfg := config.LoadConfig()

	// Инициализация базы данных
	dbConn, err := db.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}

	// Инициализация кэша (Memcached)
	memcacheClient, err := cache.NewMemcacheClient(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to Memcached: %v", err)
	}

	// Инициализация Redis
	redisClient := cache.NewRedisClient(cfg.RedisHost, cfg.RedisPort)

	// Инициализация Kafka Producer
	kafkaProducer, err := messaging.NewKafkaProducer(cfg.KafkaBroker)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}

	// Инициализация Telegram бота
	telegramBot, err := bot.NewBot(cfg.TelegramToken)
	if err != nil {
		log.Fatalf("Failed to create Telegram Bot: %v", err)
	}

	// Инициализация Kafka Consumer с Redis и Telegram ботом
	kafkaConsumer, err := messaging.NewKafkaConsumer(cfg.KafkaBroker, "search_group", redisClient, telegramBot)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka consumer: %v", err)
	}

	// Создание репозитория пользователей
	userRepo := repository.NewUserRepository(dbConn.Conn)

	// Инициализация хендлеров (обработчики команд и сообщений)
	updateHandler := handlers.NewUpdateHandler(telegramBot, userRepo, memcacheClient, redisClient, kafkaProducer)

	// Запуск бота и Kafka consumer
	go kafkaConsumer.HandleSearchRequests() // Запуск Kafka потребителя для обработки запросов
	bot.Start(telegramBot, updateHandler)
}

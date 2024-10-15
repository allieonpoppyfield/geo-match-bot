package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
	PostgresUser  string
	PostgresPass  string
	PostgresDB    string
	PostgresHost  string
	MemcacheHost  string
	MemcachePort  string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return &Config{
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		PostgresUser:  os.Getenv("POSTGRES_USER"),
		PostgresPass:  os.Getenv("POSTGRES_PASSWORD"),
		PostgresDB:    os.Getenv("POSTGRES_DB"),
		PostgresHost:  os.Getenv("POSTGRES_HOST"),
		MemcacheHost:  os.Getenv("MEMCACHE_HOST"),
		MemcachePort:  os.Getenv("MEMCACHE_PORT"),
	}
}

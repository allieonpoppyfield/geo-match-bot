package messaging

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaProducer struct {
	producer *kafka.Producer
}

type KafkaConsumer struct {
	consumer    *kafka.Consumer
	redisClient *cache.RedisClient // Redis для поиска по геолокации
	bot         *tgbotapi.BotAPI   // Telegram Bot для отправки сообщений
}

// NewKafkaProducer создает новый продюсер Kafka
func NewKafkaProducer(broker string) (*KafkaProducer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
	})
	if err != nil {
		return nil, err
	}

	return &KafkaProducer{producer: p}, nil
}

// Produce отправляет сообщение в Kafka
func (kp *KafkaProducer) Produce(topic string, key string, value string) error {
	err := kp.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          []byte(value),
	}, nil)

	if err != nil {
		log.Println("Failed to produce message:", err)
		return err
	}

	kp.producer.Flush(15 * 1000)
	return nil
}

func NewKafkaConsumer(broker, groupID string, redisClient *cache.RedisClient, bot *tgbotapi.BotAPI) (*KafkaConsumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"group.id":          groupID,
		"auto.offset.reset": "earliest",
	})

	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		consumer:    c,
		redisClient: redisClient,
		bot:         bot, // Прокидываем Telegram бот
	}, nil
}

// Subscribe подписывается на топик и начинает получать сообщения
func (kc *KafkaConsumer) Subscribe(topic string) error {
	err := kc.consumer.Subscribe(topic, nil)
	if err != nil {
		return err
	}

	for {
		msg, err := kc.consumer.ReadMessage(-1)
		if err == nil {
			fmt.Printf("Received message: %s\n", string(msg.Value))
		} else {
			fmt.Printf("Consumer error: %v\n", err)
		}
	}
}

func (kc *KafkaConsumer) HandleSearchRequests() {
	err := kc.consumer.Subscribe("search_requests", nil)
	if err != nil {
		log.Fatalf("Error subscribing to search_requests: %v", err)
	}

	for {
		msg, err := kc.consumer.ReadMessage(-1)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			continue
		}

		// Обрабатываем запрос на поиск
		parts := strings.Split(string(msg.Value), ",")
		telegramID, _ := strconv.ParseInt(parts[0], 10, 64)
		latitude, _ := strconv.ParseFloat(parts[1], 64)
		longitude, _ := strconv.ParseFloat(parts[2], 64)

		// Ищем пользователей в Redis через redisClient
		nearbyUsers, err := kc.redisClient.FindNearbyUsers(latitude, longitude, 3.0) // Радиус поиска 3 км
		if err != nil {
			log.Printf("Error finding nearby users: %v", err)
			continue
		}

		// Отправляем найденных пользователей обратно в бот
		kc.SendSearchResults(telegramID, nearbyUsers)
	}
}

func (kc *KafkaConsumer) SendSearchResults(telegramID int64, nearbyUsers []string) {
	if len(nearbyUsers) == 0 {
		// Если пользователей не найдено, отправляем уведомление
		kc.bot.Send(tgbotapi.NewMessage(telegramID, "К сожалению, поблизости не найдено пользователей для общения. Попробуйте позже."))
		return
	}

	// Формируем сообщение с найденными пользователями
	msgText := "Найдены следующие пользователи в вашем радиусе:\n"
	for _, userID := range nearbyUsers {
		msgText += fmt.Sprintf("Пользователь с ID: %s\n", userID) // Здесь можно расширить информацию о пользователе
	}

	// Отправляем сообщение пользователю
	msg := tgbotapi.NewMessage(telegramID, msgText)
	_, err := kc.bot.Send(msg)
	if err != nil {
		log.Printf("Error sending search results to user: %v", err)
	}
}

package messaging

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/repository"
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
	consumer       *kafka.Consumer
	redisClient    *cache.RedisClient // Redis для поиска по геолокации
	bot            *tgbotapi.BotAPI   // Telegram Bot для отправки сообщений
	userRepository *repository.UserRepository
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

func NewKafkaConsumer(broker, groupID string, redisClient *cache.RedisClient, bot *tgbotapi.BotAPI, userRepo *repository.UserRepository) (*KafkaConsumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"group.id":          groupID,
		"auto.offset.reset": "earliest",
	})

	if err != nil {
		return nil, err
	}

	return &KafkaConsumer{
		consumer:       c,
		redisClient:    redisClient,
		bot:            bot,
		userRepository: userRepo, // Добавлено!
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

// HandleSearchRequests обрабатывает поисковые запросы
func (kc *KafkaConsumer) HandleSearchRequests() {
	err := kc.consumer.Subscribe("geo-match-search", nil)
	if err != nil {
		log.Fatalf("Error subscribing to geo-match-search: %v", err)
	}
	return
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
		nearbyUsers, err := kc.redisClient.FindNearbyUsers(telegramID, latitude, longitude, 3.0) // Радиус поиска 3 км
		if err != nil {
			log.Printf("Error finding nearby users: %v", err)
			continue
		}

		// Отправляем найденных пользователей обратно в бот
		kc.SendSearchResults(telegramID, nearbyUsers)
	}
}

// SendSearchResults отправляет результаты поиска пользователю
func (kc *KafkaConsumer) SendSearchResults(telegramID int64, nearbyUsers []string) {
	if len(nearbyUsers) == 0 {
		// Если пользователей не найдено, отправляем уведомление
		kc.bot.Send(tgbotapi.NewMessage(telegramID, "К сожалению, поблизости не найдено пользователей для общения. Попробуйте позже."))
		return
	}

	// Для каждого найденного пользователя вызываем метод показа профиля
	for _, nearbyUserID := range nearbyUsers {
		kc.SendProfileToUser(telegramID, nearbyUserID)
	}
}

// SendProfileToUser отправляет профиль найденного пользователя в бота
func (kc *KafkaConsumer) SendProfileToUser(requesterTelegramID int64, targetUserID string) {
	targetUserIDInt, err := strconv.Atoi(targetUserID)
	if err != nil {
		log.Println(err)
	}
	// Получаем данные пользователя через репозиторий (или кеш)
	user, err := kc.userRepository.GetUserByTelegramID(int64(targetUserIDInt))
	if err != nil || user == nil {
		kc.bot.Send(tgbotapi.NewMessage(requesterTelegramID, "Ошибка при получении данных профиля пользователя."))
		return
	}
	// Формируем текст профиля
	profileText := fmt.Sprintf("Имя: %s\nВозраст: %d\nПол: %s\nО себе: %s",
		user.FirstName, user.Age, user.Gender, user.Bio)

	// Отправляем текст профиля
	msg := tgbotapi.NewMessage(requesterTelegramID, profileText)
	kc.bot.Send(msg)

	// Получаем фото пользователя
	photo, err := kc.userRepository.GetUserPhoto(int64(targetUserIDInt))
	if err == nil && photo != "" {
		photoMsg := tgbotapi.NewPhoto(requesterTelegramID, tgbotapi.FileID(photo))
		kc.bot.Send(photoMsg)
	}

	// Добавляем inline-кнопки для взаимодействия
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Предложить пообщаться", fmt.Sprintf("connect_%s", targetUserID)),
			tgbotapi.NewInlineKeyboardButtonData("Искать дальше", "search_next"),
		),
	)

	menuMsg := tgbotapi.NewMessage(requesterTelegramID, "Что вы хотите сделать?")
	menuMsg.ReplyMarkup = keyboard
	kc.bot.Send(menuMsg)
}

package handlers

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/fsm"
	"geo_match_bot/internal/messaging"
	"geo_match_bot/internal/repository"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler struct {
	bot            *tgbotapi.BotAPI
	userRepository *repository.UserRepository
	cache          *cache.MemcacheClient
	fsm            *fsm.FSM
	redisClient    *cache.RedisClient       // Добавляем RedisClient
	kafkaProducer  *messaging.KafkaProducer // Добавляем KafkaProducer
}

func NewUpdateHandler(
	bot *tgbotapi.BotAPI,
	userRepo *repository.UserRepository,
	cache *cache.MemcacheClient,
	redisClient *cache.RedisClient, // Добавляем RedisClient
	kafkaProducer *messaging.KafkaProducer, // Добавляем KafkaProducer
) func(update tgbotapi.Update) {
	fsmHandler := fsm.NewFSM(cache)
	handler := &UpdateHandler{
		bot:            bot,
		userRepository: userRepo,
		cache:          cache,
		fsm:            fsmHandler,
		redisClient:    redisClient,   // Инициализация RedisClient
		kafkaProducer:  kafkaProducer, // Инициализация KafkaProducer
	}
	return handler.HandleUpdate
}

func (h *UpdateHandler) HandleUpdate(update tgbotapi.Update) {
	// Обрабатываем callback query (нажатие на inline-кнопки)
	if update.CallbackQuery != nil {
		h.HandleCallbackQuery(update.CallbackQuery)
		return
	}
	// Обрабатываем команды (например, /start)
	if update.Message != nil {
		if update.Message.IsCommand() {
			h.HandleCommand(update)
		} else {
			h.HandleMessage(update) // обработка текстовых сообщений
		}
	}
}

func (h *UpdateHandler) ShowMainMenu(telegramID int64) {
	commands, txt := fsm.GetCommandsInstance().MainMenu()
	// Отправляем команды боту
	_, err := h.bot.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Panic(err)
	}
	msg := tgbotapi.NewMessage(telegramID, txt)
	msg.ParseMode = "HTML"
	h.bot.Send(msg)
}

func (h *UpdateHandler) ToggleVisibility(update tgbotapi.Update, telegramID int64, currentVisibility string) {
	newVisibility := "true"
	if currentVisibility == "true" {
		newVisibility = "false"
	}

	// Обновляем статус видимости в кэше
	h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), newVisibility)

	// Обновляем данные в Redis и Kafka в зависимости от нового статуса
	if newVisibility == "true" {
		// Проверяем, есть ли геолокация пользователя
		latitude, longitude, err := h.redisClient.GetUserLocation(telegramID)
		if err != nil || latitude == 0 || longitude == 0 {
			// Если геолокации нет или она некорректна, запрашиваем у пользователя
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Включение видимости требует указания геолокации. Пожалуйста, отправьте свою геопозицию."))
			h.fsm.SetState(telegramID, fsm.StepSetLocationForVisibility) // Состояние для получения геолокации
			return
		}

		// Добавляем пользователя в Redis и Kafka
		h.redisClient.AddUserLocation(telegramID, latitude, longitude)
		h.kafkaProducer.Produce("geo-match-search", "user_visibility", fmt.Sprintf("%d,%f,%f", telegramID, latitude, longitude))
	} else {
		// Удаляем пользователя из Redis и Kafka
		h.redisClient.RemoveUserLocation(telegramID)
		h.kafkaProducer.Produce("geo-match-search", "user_remove", fmt.Sprintf("%d", telegramID))
	}

	// Обновляем кнопки в главном меню
	visibilityText := "Включена"
	if newVisibility == "false" {
		visibilityText = "Выключена"
	}
	msg := tgbotapi.NewMessage(telegramID, fmt.Sprintf("Видимость: %s", visibilityText))
	h.ShowMainMenu(telegramID) // Переносим пользователя в главное меню
	h.bot.Send(msg)
}

func (h *UpdateHandler) saveLocationForVisibility(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID

	if update.Message.Location == nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте корректную геолокацию."))
		return
	}

	latitude := update.Message.Location.Latitude
	longitude := update.Message.Location.Longitude

	// Сохраняем локацию пользователя в Redis
	err := h.redisClient.AddUserLocation(telegramID, latitude, longitude)
	if err != nil {
		log.Printf("Error saving user location in Redis: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при сохранении локации. Попробуйте позже."))
		return
	}

	// Устанавливаем пользователя как видимого
	h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), "true")
	h.kafkaProducer.Produce("geo-match-search", "user_visibility", fmt.Sprintf("%d,%f,%f", telegramID, latitude, longitude))

	// Сообщаем пользователю об успешном включении видимости и возвращаем в главное меню
	h.bot.Send(tgbotapi.NewMessage(telegramID, "Видимость успешно включена."))
	h.ShowMainMenu(telegramID)
	h.fsm.ClearState(telegramID)
}

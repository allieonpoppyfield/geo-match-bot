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
	// Получаем текущий статус видимости пользователя
	visibility, err := h.cache.Get(fmt.Sprintf("visibility:%d", telegramID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении статуса видимости. Попробуйте позже."))
		return
	}

	// Определяем текст кнопки на основе текущего статуса
	visibilityText := "Видимость: Включена"
	if visibility == "" || visibility == "false" {
		visibilityText = "Видимость: Отключена"
	}

	msg := tgbotapi.NewMessage(telegramID, "Главное меню")

	// Создаем inline-кнопки "Профиль", "Начать поиск" и "Видимость"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Профиль", "profile"),
			tgbotapi.NewInlineKeyboardButtonData("Начать поиск", "start_search"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(visibilityText, "toggle_visibility"),
		),
	)
	msg.ReplyMarkup = keyboard

	h.bot.Send(msg)
}

func (h *UpdateHandler) ShowUserProfile(telegramID int64) {
	// Получаем данные пользователя
	user, err := h.userRepository.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении данных профиля."))
		return
	}

	// Формируем сообщение с информацией о пользователе
	profileText := fmt.Sprintf("Ваш профиль:\nИмя: %s\nВозраст: %d\nПол: %s\nО себе: %s",
		user.FirstName, user.Age, user.Gender, user.Bio)

	msg := tgbotapi.NewMessage(telegramID, profileText)

	// Отправляем текст профиля
	h.bot.Send(msg)

	// Получаем фото пользователя из таблицы photos
	photo, err := h.userRepository.GetUserPhoto(telegramID)
	if err == nil && photo != "" {
		// Если фото найдено, отправляем его
		photoMsg := tgbotapi.NewPhoto(telegramID, tgbotapi.FileID(photo))
		h.bot.Send(photoMsg)
	}

	// Добавляем inline-кнопки "Редактировать профиль" и "Назад"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Редактировать профиль", "edit_profile"),
			tgbotapi.NewInlineKeyboardButtonData("Назад", "back_to_menu"),
		),
	)
	menuMsg := tgbotapi.NewMessage(telegramID, "Что вы хотите сделать дальше?")
	menuMsg.ReplyMarkup = keyboard

	h.bot.Send(menuMsg)
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

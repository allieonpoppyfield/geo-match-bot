package handlers

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/fsm"
	"geo_match_bot/internal/messaging"
	"geo_match_bot/internal/repository"

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
	msg := tgbotapi.NewMessage(telegramID, "Главное меню")

	// Создаем inline-кнопки "Профиль" и "Начать поиск"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Профиль", "profile"),
			tgbotapi.NewInlineKeyboardButtonData("Начать поиск", "start_search"),
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

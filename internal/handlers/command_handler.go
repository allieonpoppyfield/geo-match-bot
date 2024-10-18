package handlers

import (
	"geo_match_bot/internal/fsm"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CommandHandler interface {
	HandleCommand(update tgbotapi.Update)
	HandleUnknownCommand(update tgbotapi.Update)
	HandleStart(update tgbotapi.Update)
}

func (h *UpdateHandler) HandleCommand(update tgbotapi.Update) {
	switch update.Message.Command() {
	case "start":
		h.HandleStart(update)
	default:
		h.HandleUnknownCommand(update)
	}
}

func (h *UpdateHandler) HandleUnknownCommand(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	msg := tgbotapi.NewMessage(telegramID, "Неизвестная команда. Попробуйте использовать /start.")
	h.bot.Send(msg)
}

func (h *UpdateHandler) HandleStart(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	username := update.Message.From.UserName
	firstName := update.Message.From.FirstName
	lastName := update.Message.From.LastName

	// Проверяем, существует ли профиль пользователя
	user, err := h.userRepository.GetUserByTelegramID(telegramID)
	if err != nil {
		log.Printf("Ошибка при проверке профиля пользователя: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при проверке вашего профиля. Попробуйте позже."))
		return
	}

	if user == nil {
		// Если пользователя нет, создаем новый профиль
		err = h.userRepository.CreateUser(telegramID, username, firstName, lastName)
		if err != nil {
			log.Printf("Ошибка при создании профиля пользователя: %v", err)
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Не удалось создать ваш профиль. Попробуйте позже."))
			return
		}
		// Начинаем процесс заполнения профиля с вопроса о поле
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Добро пожаловать! Укажите ваш пол (м/ж):"))
		h.fsm.SetState(telegramID, fsm.StepGender) // Переход к шагу выбора пола
		return
	}

	// Если профиль уже существует, показываем главное меню
	h.ShowMainMenu(telegramID)
}

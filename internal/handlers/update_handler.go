package handlers

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/fsm"
	"geo_match_bot/internal/repository"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler struct {
	bot            *tgbotapi.BotAPI
	userRepository *repository.UserRepository
	cache          *cache.MemcacheClient
	fsm            *fsm.FSM
}

func NewUpdateHandler(bot *tgbotapi.BotAPI, userRepo *repository.UserRepository, cache *cache.MemcacheClient) func(update tgbotapi.Update) {
	fsmHandler := fsm.NewFSM(cache)
	handler := &UpdateHandler{
		bot:            bot,
		userRepository: userRepo,
		cache:          cache,
		fsm:            fsmHandler,
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

	// Проверяем, существует ли профиль пользователя
	user, err := h.userRepository.GetUserByTelegramID(telegramID)
	if err != nil {
		log.Printf("Ошибка при проверке профиля пользователя: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при проверке вашего профиля. Попробуйте позже."))
		return
	}

	if user != nil {
		// Если профиль существует, показываем главное меню
		h.ShowMainMenu(telegramID)
		return
	}

	// Если профиля нет, начинаем процесс заполнения
	msg := tgbotapi.NewMessage(telegramID, "Добро пожаловать! Укажите ваш пол (мужской/женский):")
	h.bot.Send(msg)

	// Устанавливаем начальный шаг
	h.fsm.SetState(telegramID, fsm.StepGender)
}

// Обработка сообщений (ответов на вопросы)
func (h *UpdateHandler) HandleMessage(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	currentState, err := h.fsm.GetState(telegramID)
	if err != nil {
		log.Printf("Error retrieving state: %v", err)
		return
	}

	switch currentState {
	case fsm.StepGender:
		h.saveGender(update)
	case fsm.StepAge:
		h.saveAge(update)
	case fsm.StepBio:
		h.saveBio(update)
	case fsm.StepPhoto:
		h.savePhoto(update)
	default:
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, начните с команды /start."))
	}
}
func (h *UpdateHandler) saveGender(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	gender := update.Message.Text

	// Проверяем, что гендер либо "м" (мужской), либо "ж" (женский)
	if !strings.EqualFold(gender, "м") && !strings.EqualFold(gender, "ж") {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, введите 'м' для мужского или 'ж' для женского пола."))
		return
	}

	// Сохраняем пол в профиле пользователя
	err := h.userRepository.UpdateUserGender(telegramID, gender)
	if err != nil {
		log.Printf("Error updating gender: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при сохранении пола. Попробуйте еще раз."))
		return
	}

	// Переходим на следующий шаг
	nextState, err := h.fsm.NextStep(telegramID)
	if err != nil {
		log.Printf("Error transitioning to next state: %v", err)
		return
	}

	if nextState == fsm.StepAge {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Укажите ваш возраст:"))
	}
}

func (h *UpdateHandler) saveAge(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	age, err := strconv.Atoi(update.Message.Text)
	if err != nil || age < 14 || age > 80 {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, укажите корректный возраст (от 14 до 80 лет)."))
		return
	}

	// Сохраняем возраст в профиле пользователя
	err = h.userRepository.UpdateUserAge(telegramID, age)
	if err != nil {
		log.Printf("Error updating age: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при сохранении возраста. Попробуйте еще раз."))
		return
	}

	// Переходим к следующему шагу
	nextState, err := h.fsm.NextStep(telegramID)
	if err != nil {
		log.Printf("Error transitioning to next state: %v", err)
		return
	}

	if nextState == fsm.StepBio {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Расскажите немного о себе:"))
	}
}

func (h *UpdateHandler) saveBio(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	bio := update.Message.Text

	// Сохраняем "о себе" в профиле пользователя
	err := h.userRepository.UpdateUserBio(telegramID, bio)
	if err != nil {
		log.Printf("Error updating bio: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при сохранении описания. Попробуйте еще раз."))
		return
	}

	// Переходим к следующему шагу
	nextState, err := h.fsm.NextStep(telegramID)
	if err != nil {
		log.Printf("Error transitioning to next state: %v", err)
		return
	}

	if nextState == fsm.StepPhoto {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте фото для верификации:"))
	}
}
func (h *UpdateHandler) savePhoto(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID

	if update.Message.Photo == nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте корректное фото."))
		return
	}

	// Получаем самое высокое по качеству фото
	photo := update.Message.Photo[len(update.Message.Photo)-1]
	fileID := photo.FileID

	// Получаем внутренний user_id по telegram_id
	userID, err := h.userRepository.GetUserIDByTelegramID(telegramID)
	if err != nil || userID == 0 {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при поиске вашего профиля. Попробуйте позже."))
		log.Printf("Error retrieving user ID: %v", err)
		return
	}

	// Сохраняем фото в таблицу photos
	err = h.userRepository.AddPhotoForUser(userID, fileID)
	if err != nil {
		log.Printf("Error saving photo: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Произошла ошибка при сохранении фото. Попробуйте еще раз."))
		return
	}

	// Завершаем процесс создания профиля
	h.bot.Send(tgbotapi.NewMessage(telegramID, "Ваш профиль успешно создан и отправлен на проверку."))
	h.fsm.ClearState(telegramID)
}

func (h *UpdateHandler) ShowMainMenu(telegramID int64) {
	msg := tgbotapi.NewMessage(telegramID, "Главное меню")

	// Создаем inline-кнопку "Профиль"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Профиль", "profile"),
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

func (h *UpdateHandler) HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	telegramID := callbackQuery.Message.Chat.ID

	switch callbackQuery.Data {
	case "profile":
		h.ShowUserProfile(telegramID)
	case "edit_profile":
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Функция редактирования профиля пока не реализована."))
	case "back_to_menu":
		h.ShowMainMenu(telegramID)
	default:
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Неизвестная команда."))
	}
}

package handlers

import (
	"fmt"
	"geo_match_bot/internal/fsm"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MessageHandler interface {
	HandleMessage(update tgbotapi.Update)
} 

// Обработка сообщений (ответов на вопросы)
func (h *UpdateHandler) HandleMessage(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	currentState, err := h.fsm.GetState(telegramID)
	if err != nil {
		log.Printf("Error retrieving state: %v", err)
		return
	}

	// Обработка состояния установки геолокации для видимости
	if currentState == fsm.StepSetLocationForVisibility {
		if update.Message.Location == nil {
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте корректную геолокацию."))
			return
		}

		latitude := update.Message.Location.Latitude
		longitude := update.Message.Location.Longitude

		// Сохраняем локацию пользователя в Redis и включаем видимость
		err := h.redisClient.AddUserLocation(telegramID, latitude, longitude)
		if err != nil {
			log.Printf("Error saving user location in Redis: %v", err)
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при сохранении локации. Попробуйте позже."))
			return
		}

		// Добавляем пользователя в Kafka
		h.kafkaProducer.Produce("geo-match-search", "user_visibility", fmt.Sprintf("%d,%f,%f", telegramID, latitude, longitude))

		// Устанавливаем видимость в кэше
		h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), "true")

		// Завершаем установку и очищаем состояние FSM
		h.fsm.ClearState(telegramID)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Видимость включена. Теперь вы доступны для поиска."))
		return
	}

	if update.Message.Text == "Завершить общение" {
		chatPartner, innerErr := h.cache.Get(fmt.Sprintf("chat:%d", telegramID))
		if innerErr == nil && chatPartner != "" {
			partnerID, _ := strconv.ParseInt(chatPartner, 10, 64)
			h.EndChat(telegramID, partnerID) // Завершение чата для обоих
		}
		return
	}

	// Проверяем, находится ли пользователь в чате
	chatPartner, err := h.cache.Get(fmt.Sprintf("chat:%d", telegramID))
	if err == nil && chatPartner != "" {
		partnerID, _ := strconv.ParseInt(chatPartner, 10, 64)

		// Проверка, если пользователь отправляет текстовое сообщение
		if update.Message.Text != "" {
			msg := tgbotapi.NewMessage(partnerID, update.Message.Text)
			h.bot.Send(msg)
			return
		}

		// Проверка, если пользователь отправляет фото
		if update.Message.Photo != nil {
			// Получаем самое высокое по качеству фото
			photo := update.Message.Photo[len(update.Message.Photo)-1]
			photoMsg := tgbotapi.NewPhoto(partnerID, tgbotapi.FileID(photo.FileID))
			h.bot.Send(photoMsg)
			return
		}
	}

	switch currentState {
	case fsm.StepTitleName:
		h.saveTitleName(update)
	case fsm.StepGender:
		h.saveGender(update)
	case fsm.StepAge:
		h.saveAge(update)
	case fsm.StepBio:
		h.saveBio(update)
	case fsm.StepPhoto:
		h.savePhoto(update)
	case fsm.StepSearchGender: // Добавляем новый шаг поиска
		h.saveSearchGender(update)
	case fsm.StepSearchAge: // Добавляем шаг для сохранения возраста в поиске
		h.saveSearchAge(update)
	case fsm.StepSearchLocation: // Добавляем шаг для обработки локации
		h.saveSearchLocation(update)
	default:
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, начните с команды /start."))
	}
}

func (h *UpdateHandler) saveTitleName(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	titleName := update.Message.Text

	// Проверяем, что гендер либо "м" (мужской), либо "ж" (женский)
	if len(titleName) > 50 {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, введите корректное имя. Вы прислали слишком много символов."))
		return
	}

	// Сохраняем пол в профиле пользователя
	err := h.userRepository.UpdateUserTitleName(telegramID, titleName)
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

	if nextState == fsm.StepGender {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Укажите ваш пол (м/ж):"))
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
	h.ShowMainMenu(telegramID)
}

func (h *UpdateHandler) saveSearchGender(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	gender := update.Message.Text

	if !strings.EqualFold(gender, "м") && !strings.EqualFold(gender, "ж") {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, укажите корректный пол ('м' или 'ж')."))
		return
	}

	// Сохраняем пол в кэш или базу
	h.cache.Set(fmt.Sprintf("search_gender:%d", telegramID), gender)

	// Переходим к следующему шагу: возраст
	h.fsm.SetState(telegramID, fsm.StepSearchAge)
	h.bot.Send(tgbotapi.NewMessage(telegramID, "Укажите желаемый возрастной диапазон (например, 25-30):"))
}

func (h *UpdateHandler) saveSearchAge(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	ageRange := update.Message.Text

	// Сохраняем возраст в кэш или базу
	h.cache.Set(fmt.Sprintf("search_age:%d", telegramID), ageRange)

	// Переходим к следующему шагу: запрос геолокации
	h.fsm.SetState(telegramID, fsm.StepSearchLocation)
	h.bot.Send(tgbotapi.NewMessage(telegramID, "Отправьте свою геолокацию:"))
}

func (h *UpdateHandler) saveSearchLocation(update tgbotapi.Update) {
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

	// Отправляем запрос на поиск через Kafka
	h.StartKafkaSearch(telegramID, latitude, longitude)
}

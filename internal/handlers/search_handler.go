package handlers

import (
	"fmt"
	"geo_match_bot/internal/fsm"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type SearchHandler interface {
	StartSearchProcess(telegramID int64)
	StartSearch(update tgbotapi.Update)
	StartKafkaSearch(telegramID int64, latitude, longitude float64)
	ShowNearbyUser(telegramID int64, userID string)
	SendProfileToUser(senderID int64, targetUserID string)
	SearchNextUser(telegramID int64)
}

func (h *UpdateHandler) StartSearchProcess(telegramID int64) {
	// Сообщаем пользователю, что начался поиск
	msg := tgbotapi.NewMessage(telegramID, "Начинаем поиск...")
	h.bot.Send(msg)

	// Запрашиваем у пользователя его местоположение (если не было запрошено ранее)
	h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте свою геолокацию:"))
	h.fsm.SetState(telegramID, fsm.StepSearchLocation)
}
func (h *UpdateHandler) StartSearch(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID

	// Запрашиваем пол пользователя
	msg := tgbotapi.NewMessage(telegramID, "Кого вы хотите найти? Укажите пол (м/ж):")
	h.bot.Send(msg)

	// Устанавливаем состояние FSM
	h.fsm.SetState(telegramID, fsm.StepSearchGender)
}
func (h *UpdateHandler) StartKafkaSearch(telegramID int64, latitude, longitude float64) {
	// Отправляем запрос на поиск через Kafka
	err := h.kafkaProducer.Produce("geo-match-search", "user_search", fmt.Sprintf("%d,%f,%f", telegramID, latitude, longitude))
	if err != nil {
		log.Printf("Error sending search request to Kafka: %v", err)
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при запуске поиска. Попробуйте позже."))
		return
	}

	h.bot.Send(tgbotapi.NewMessage(telegramID, "Начат поиск пользователей поблизости... Ожидайте результатов."))
}
func (h *UpdateHandler) ShowNearbyUser(telegramID int64, userID string) {
	// Получаем данные пользователя по его внутреннему ID
	user, err := h.userRepository.GetUserByID(userID)
	if err != nil || user == nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении данных пользователя."))
		return
	}

	// Формируем текст профиля
	profileText := fmt.Sprintf("Имя: %s\nВозраст: %d\nПол: %s\nО себе: %s",
		user.FirstName, user.Age, user.Gender, user.Bio)

	msg := tgbotapi.NewMessage(telegramID, profileText)
	h.bot.Send(msg)

	// Получаем фото пользователя
	photo, err := h.userRepository.GetUserPhotoByID(userID)
	if err == nil && photo != "" {
		photoMsg := tgbotapi.NewPhoto(telegramID, tgbotapi.FileID(photo))
		h.bot.Send(photoMsg)
	}

	// Добавляем inline-кнопки "Предложить пообщаться" и "Искать дальше"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Предложить пообщаться", fmt.Sprintf("connect_%s", userID)),
			tgbotapi.NewInlineKeyboardButtonData("Искать дальше", "search_next"),
		),
	)

	menuMsg := tgbotapi.NewMessage(telegramID, "Что вы хотите сделать?")
	menuMsg.ReplyMarkup = keyboard
	h.bot.Send(menuMsg)
}
func (h *UpdateHandler) SendProfileToUser(senderID int64, targetUserID string) {
	// Получаем данные пользователя, который хочет пообщаться
	senderProfile, err := h.userRepository.GetUserByTelegramID(senderID)
	if err != nil || senderProfile == nil {
		h.bot.Send(tgbotapi.NewMessage(senderID, "Ошибка при получении вашего профиля."))
		return
	}

	// Формируем сообщение для целевого пользователя
	profileText := fmt.Sprintf("Пользователь %s хочет с вами пообщаться:\nИмя: %s\nВозраст: %d\nПол: %s\nО себе: %s",
		senderProfile.FirstName, senderProfile.FirstName, senderProfile.Age, senderProfile.Gender, senderProfile.Bio)

	// Отправляем сообщение целевому пользователю
	targetUserIDInt, _ := strconv.ParseInt(targetUserID, 10, 64)

	// Получаем фото отправителя из репозитория
	photo, err := h.userRepository.GetUserPhoto(senderID)
	if err == nil && photo != "" {
		// Если фото найдено, отправляем его
		photoMsg := tgbotapi.NewPhoto(targetUserIDInt, tgbotapi.FileID(photo))
		h.bot.Send(photoMsg)
	}

	// Отправляем текст профиля
	msg := tgbotapi.NewMessage(targetUserIDInt, profileText)
	h.bot.Send(msg)

	// Кнопки для ответа
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Принять", fmt.Sprintf("accept_%d", senderID)),
			tgbotapi.NewInlineKeyboardButtonData("Отказать", fmt.Sprintf("decline_%d", senderID)),
		),
	)

	menuMsg := tgbotapi.NewMessage(targetUserIDInt, "Что вы хотите сделать?")
	menuMsg.ReplyMarkup = keyboard
	h.bot.Send(menuMsg)
}
func (h *UpdateHandler) SearchNextUser(telegramID int64) {
	// Получаем текущую локацию пользователя
	latitude, longitude, err := h.redisClient.GetUserLocation(telegramID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении вашей локации. Попробуйте позже."))
		return
	}

	// Ищем пользователей поблизости в радиусе, скажем, 10 км
	nearbyUsers, err := h.redisClient.FindNearbyUsers(telegramID, latitude, longitude, 10)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при поиске пользователей. Попробуйте позже."))
		return
	}

	if len(nearbyUsers) == 0 {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Поблизости нет пользователей."))
		return
	}

	// Показываем первого найденного пользователя
	h.ShowNearbyUser(telegramID, nearbyUsers[0])
}

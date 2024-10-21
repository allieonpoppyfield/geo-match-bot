package handlers

import (
	"fmt"
	"geo_match_bot/internal/fsm"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CallbackHandler interface {
	HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery)
}

func (h *UpdateHandler) HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	telegramID := callbackQuery.Message.Chat.ID

	if callbackQuery.Data == "toggle_visibility" {
		// Получаем текущий статус видимости пользователя
		currentVisibility, err := h.cache.Get(fmt.Sprintf("visibility:%d", telegramID))
		if err != nil || currentVisibility == "" {
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении статуса видимости. Попробуйте позже."))
			return
		}

		// Определяем новое значение видимости
		newVisibility := "true"
		if currentVisibility == "true" {
			newVisibility = "false"
		}

		// Если видимость включается, запрашиваем локацию
		if newVisibility == "true" {
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Пожалуйста, отправьте свою геолокацию для включения видимости."))
			h.fsm.SetState(telegramID, fsm.StepSetLocationForVisibility)
		} else {
			// Если видимость отключается, обновляем Redis и Kafka
			h.redisClient.RemoveUserLocation(telegramID)
			h.kafkaProducer.Produce("geo-match-search", "user_remove", fmt.Sprintf("%d", telegramID))
			h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), newVisibility)
			h.bot.Send(tgbotapi.NewMessage(telegramID, "Видимость выключена."))
		}
		return
	}

	// Проверяем нажатие кнопки на предложение общения
	if strings.HasPrefix(callbackQuery.Data, "connect_") {
		// Извлекаем ID целевого пользователя
		targetUserID := strings.TrimPrefix(callbackQuery.Data, "connect_")
		h.SendProfileToUser(telegramID, targetUserID)
		return
	}

	// Обработка принятия или отказа от общения
	if strings.HasPrefix(callbackQuery.Data, "accept_") {
		// Извлекаем ID пользователя, отправившего запрос
		senderID := strings.TrimPrefix(callbackQuery.Data, "accept_")
		h.StartChat(telegramID, senderID)
		return
	}

	if strings.HasPrefix(callbackQuery.Data, "decline_") {
		// Обработка отказа от общения
		senderID := strings.TrimPrefix(callbackQuery.Data, "decline_")
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Вы отказали в общении."))
		senderIDInt64, _ := strconv.ParseInt(senderID, 10, 64)
		h.bot.Send(tgbotapi.NewMessage(senderIDInt64, "Ваш запрос на общение был отклонен."))
		return
	}

	// Стандартные действия для других кнопок
	switch callbackQuery.Data {
	case "edit_profile":
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Функция редактирования профиля пока не реализована."))
	case "back_to_menu":
		h.ShowMainMenu(telegramID)
	case "start_search": // Обработка кнопки "Начать поиск"
		h.StartSearchProcess(telegramID)
	case "search_next":
		h.SearchNextUser(telegramID)
	default:
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Неизвестная команда."))
	}
}

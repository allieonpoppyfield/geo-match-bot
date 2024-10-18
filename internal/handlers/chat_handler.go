package handlers

import (
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ChatHandler interface {
	StartChat(userID1 int64, userID2 string)
	EndChat(userID1, userID2 int64)
}

func (h *UpdateHandler) StartChat(userID1 int64, userID2 string) {
	// Переводим двух пользователей в режим общения
	h.cache.Set(fmt.Sprintf("chat:%d", userID1), userID2)
	h.cache.Set(fmt.Sprintf("chat:%s", userID2), strconv.FormatInt(userID1, 10))

	// Кнопка завершения чата для пользователя 1
	keyboard1 := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Завершить общение"),
		),
	)

	// Кнопка завершения чата для пользователя 2
	keyboard2 := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Завершить общение"),
		),
	)

	// Уведомляем пользователей о начале общения
	msg1 := tgbotapi.NewMessage(userID1, "Вы начали общение. Отправьте сообщение, чтобы начать переписку.")
	msg1.ReplyMarkup = keyboard1
	h.bot.Send(msg1)

	userID2Int, _ := strconv.ParseInt(userID2, 10, 64)
	msg2 := tgbotapi.NewMessage(userID2Int, "Вы начали общение. Отправьте сообщение, чтобы начать переписку.")
	msg2.ReplyMarkup = keyboard2
	h.bot.Send(msg2)
}

func (h *UpdateHandler) EndChat(userID1, userID2 int64) {
	// Удаляем информацию о чате для обоих пользователей
	h.cache.Delete(fmt.Sprintf("chat:%d", userID1))
	h.cache.Delete(fmt.Sprintf("chat:%d", userID2))

	// Уведомляем пользователей о завершении чата
	h.bot.Send(tgbotapi.NewMessage(userID1, "Чат завершён."))
	h.bot.Send(tgbotapi.NewMessage(userID2, "Чат завершён."))

	// Удаляем кнопки завершения для обоих пользователей
	msg1 := tgbotapi.NewMessage(userID1, "Вы завершили общение")
	msg1.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	h.bot.Send(msg1)

	msg2 := tgbotapi.NewMessage(userID2, "Ваш собеседник завершил общение")
	msg2.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	h.bot.Send(msg2)
}

package handlers

import (
	"fmt"
	"geo_match_bot/internal/fsm"
	"log"
	"strconv"
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CommandHandler interface {
	HandleCommand(update tgbotapi.Update)
	HandleUnknownCommand(update tgbotapi.Update)
	HandleStart(update tgbotapi.Update)
	HandleProfile(update tgbotapi.Update)
}

func (h *UpdateHandler) HandleCommand(update tgbotapi.Update) {
	switch update.Message.Command() {
	case "start":
		h.HandleStart(update)
	case "profile":
		h.HandleProfile(update)
	case "main_menu":
		h.ShowMainMenu(update.Message.Chat.ID)
	case "current_visibility":
		h.HandleCurrentVisibility(update)
	case "toogle_visibility":
		h.HandleToogleVisibility(update)
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
		// Устанавливаем пользователя как невидимого по умолчанию
		h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), "false")

		// Начинаем процесс заполнения профиля с вопроса об имени
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Добро пожаловать! Как вас зовут?"))
		h.fsm.SetState(telegramID, fsm.StepTitleName) // Переход к шагу заполнения имени
		return
	}

	// Если профиль уже существует, показываем главное меню
	h.ShowMainMenu(telegramID)
}

func (h *UpdateHandler) HandleProfile(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	// Получаем данные пользователя
	user, err := h.userRepository.GetUserByTelegramID(telegramID)
	if err != nil || user == nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, "Ошибка при получении данных профиля."))
		return
	}

	// Получаем фото пользователя из таблицы photos
	photo, err := h.userRepository.GetUserPhoto(telegramID)
	if err == nil && photo != "" {
		// Если фото найдено, отправляем его
		photoMsg := tgbotapi.NewPhoto(telegramID, tgbotapi.FileID(photo))
		h.bot.Send(photoMsg)
	}

	commands, txt := fsm.GetCommandsInstance().Profile()

	// Отправляем команды боту
	_, err = h.bot.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Panic(err)
	}

	// Формируем сообщение с информацией о пользователе
	profileText := fmt.Sprintf(
		`👤  <b>Имя:</b> %s
	    🎂 <b>Возраст:</b> %d
	    ⚤  <b>Пол:</b> %s
	    📄 <b>О себе:</b> %s%s
	`, user.TitleName, user.Age, formatGender(user.Gender), user.Bio, txt)

	msg := tgbotapi.NewMessage(telegramID, profileText)
	msg.ParseMode = "HTML"

	// Отправляем текст профиля
	h.bot.Send(msg)
}
func (h *UpdateHandler) HandleCurrentVisibility(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	// Получаем текущий статус видимости пользователя
	currentVisibilityStr, err := h.cache.Get(fmt.Sprintf("visibility:%d", telegramID))
	if err != nil {
		if err == memcache.ErrCacheMiss {
			h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), "false")
			currentVisibilityStr = "false"
		} else {
			h.bot.Send(tgbotapi.NewMessage(telegramID, fmt.Sprintf("Ошибка при получении статуса видимости. Попробуйте позже. %s", err.Error())))
			return
		}
	}
	visible, err := strconv.ParseBool(currentVisibilityStr)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, fmt.Sprintf("Ошибка при получении статуса видимости. Попробуйте позже. %s", err.Error())))
		return
	}
	txt := ""
	if visible {
		txt = `Сейчас видимость вашего профиля <b>отключена</b>, он не будет отображаться в поиске.`
	} else {
		txt = `Сейчас видимость вашего профиля <b>включена</b>, он будет отображаться в поиске.`
	}
	txt += "\nДля переключения видимости используйте команду /toogle_visibility"
	msg := tgbotapi.NewMessage(telegramID, txt)
	msg.ParseMode = "HTML"
	h.bot.Send(msg)
}

// TODO: НАДО ЗАПРАШИВАТЬ ЛОКАЦИЮ ПРИ ВКЛЮЧЕНИИ
func (h *UpdateHandler) HandleToogleVisibility(update tgbotapi.Update) {
	telegramID := update.Message.Chat.ID
	// Получаем текущий статус видимости пользователя
	currentVisibilityStr, err := h.cache.Get(fmt.Sprintf("visibility:%d", telegramID))
	if err != nil {
		if err == memcache.ErrCacheMiss {
			h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), "false")
			currentVisibilityStr = "false"
		} else {
			h.bot.Send(tgbotapi.NewMessage(telegramID, fmt.Sprintf("Ошибка при получении статуса видимости. Попробуйте позже. %s", err.Error())))
			return
		}
	}
	visible, err := strconv.ParseBool(currentVisibilityStr)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(telegramID, fmt.Sprintf("Ошибка при получении статуса видимости. Попробуйте позже. %s", err.Error())))
		return
	}

	h.cache.Set(fmt.Sprintf("visibility:%d", telegramID), strconv.FormatBool(!visible))
	var txt string
	if visible {
		txt = `Вы <b>отключили</b> видимость, ваш профиль не отображается в поиске.`
	} else {
		txt = `Вы <b>включили</b> видимость, ваш профиль отображается в поиске.`
	}
	txt += "\nДля переключения видимости используйте команду <b>/toogle_visibility</b>"
	msg := tgbotapi.NewMessage(telegramID, txt)
	msg.ParseMode = "HTML"
	h.bot.Send(msg)
}

func formatGender(gender string) string {
	switch strings.ToLower(gender) {
	case "м":
		return "Мужской ♂️"
	case "ж":
		return "Женский ♀️"
	default:
		return "Не указан"
	}
}

package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// NewBot ..
func NewBot(token string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	// Настраиваем команды для меню
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Начало работы с ботом"},
		{Command: "profile", Description: "Просмотр профиля"},
		{Command: "visibility", Description: "Переключить видимость профиля"},
		{Command: "help", Description: "Получить справку"},
	}

	// Отправляем команды боту
	_, err = bot.Request(tgbotapi.NewSetMyCommands(commands...))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)
	return bot, nil
}

func Start(bot *tgbotapi.BotAPI, updateHandler func(update tgbotapi.Update)) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		updateHandler(update)
	}
}

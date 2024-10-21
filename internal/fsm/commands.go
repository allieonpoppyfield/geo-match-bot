// commands.go (в пакете fsm)

package fsm

import (
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Commands - структура, которую мы будем использовать как синглтон
type Commands struct {
	mainMenuCommands []tgbotapi.BotCommand
	mainMenuTitle    string
	profileCommands  []tgbotapi.BotCommand
	profileTitle     string
}

var (
	commandsInstance *Commands
	once             sync.Once
)

// GetCommandsInstance возвращает синглтон экземпляр Commands
func GetCommandsInstance() *Commands {
	once.Do(func() {
		commandsInstance = &Commands{
			mainMenuCommands: mainMenuCommands,
			mainMenuTitle:    mainMenuTitle,
			profileCommands:  profileCommands,
			profileTitle:     profileTitle,
		}
	})
	return commandsInstance
}

func (c *Commands) MainMenu() ([]tgbotapi.BotCommand, string) {
	return c.mainMenuCommands, c.mainMenuTitle
}

func (c *Commands) Profile() ([]tgbotapi.BotCommand, string) {
	return c.profileCommands, c.profileTitle
}

var mainMenuCommands = []tgbotapi.BotCommand{
	{Command: "profile", Description: "Просмотр профиля"},
	{Command: "current_visibility", Description: "Текущая видимость"},
	{Command: "toggle_visibility", Description: "Включить/выключить видимость"},
	{Command: "search", Description: "Начать поиск пользователей"},
	{Command: "help", Description: "Получить справку"},
}

var mainMenuTitle = "📋 <b>Главное меню</b>\n\n" +
	"🔹 <i>Доступные команды:</i>\n" +
	"💼 <b>/profile</b> — Просмотр вашего профиля\n" +
	"👁 <b>/current_visibility</b> — Текущая видимость\n" +
	"🔄 <b>/toggle_visibility</b> — Включить/выключить видимость\n" +
	"🔍 <b>/search</b> — Начать поиск пользователей\n" +
	"ℹ️ <b>/help</b> — Получить справку\n"

var profileCommands = []tgbotapi.BotCommand{
	{Command: "edit_profile", Description: "Редактировать профиль"},
	{Command: "main_menu", Description: "Вернуться в главное меню"},
}

var profileTitle = "\n\n🔹 <i>Доступные команды:</i>\n" +
	"🔧 <b>/edit_profile</b> — Редактировать профиль\n" +
	"🏠 <b>/main_menu</b> — Вернуться в главное меню\n"

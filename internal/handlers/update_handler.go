package handlers

import (
	"fmt"
	"geo_match_bot/internal/cache"
	"geo_match_bot/internal/fsm"
	"geo_match_bot/internal/messaging"
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

func (h *UpdateHandler) HandleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	telegramID := callbackQuery.Message.Chat.ID

	if strings.HasPrefix(callbackQuery.Data, "connect_") {
		// Извлекаем ID целевого пользователя
		targetUserID := strings.TrimPrefix(callbackQuery.Data, "connect_")
		h.SendProfileToUser(telegramID, targetUserID)
		return
	}

	switch callbackQuery.Data {
	case "profile":
		h.ShowUserProfile(telegramID)
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

package repository

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

type User struct {
	ID         int
	TelegramID int64
	Username   string
	FirstName  string
	LastName   string
	Gender     string
	Age        int
	Bio        string
}

type UserRepository struct {
	db      *sql.DB
	builder sq.StatementBuilderType
}

// Конструктор для создания репозитория пользователей
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db:      db,
		builder: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// Метод для поиска пользователя по telegram_id
func (r *UserRepository) GetUserByTelegramID(telegramID int64) (*User, error) {
	query := r.builder.Select("id", "telegram_id", "username", "first_name", "last_name", "gender", "age", "bio").
		From("users").
		Where(sq.Eq{"telegram_id": telegramID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building query: %v", err)
	}

	var user User
	err = r.db.QueryRow(sqlQuery, args...).Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &user.Gender, &user.Age, &user.Bio)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &user, nil
}

// Метод для создания нового пользователя
func (r *UserRepository) CreateUser(telegramID int64, username, firstName, lastName string) error {
	query := r.builder.Insert("users").
		Columns("telegram_id", "username", "first_name", "last_name").
		Values(telegramID, username, firstName, lastName)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("error building query: %v", err)
	}

	_, err = r.db.Exec(sql, args...)
	if err != nil {
		return fmt.Errorf("error executing query: %v", err)
	}

	return nil
}

// Метод для обновления пола пользователя
func (r *UserRepository) UpdateUserGender(telegramID int64, gender string) error {
	query := r.builder.Update("users").
		Set("gender", gender).
		Where(sq.Eq{"telegram_id": telegramID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("error building query: %v", err)
	}

	_, err = r.db.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("error executing query: %v", err)
	}

	return nil
}

// Метод для обновления возраста пользователя
func (r *UserRepository) UpdateUserAge(telegramID int64, age int) error {
	query := r.builder.Update("users").
		Set("age", age).
		Where(sq.Eq{"telegram_id": telegramID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("error building query: %v", err)
	}

	_, err = r.db.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("error executing query: %v", err)
	}

	return nil
}

// Метод для обновления "о себе"
func (r *UserRepository) UpdateUserBio(telegramID int64, bio string) error {
	query := r.builder.Update("users").
		Set("bio", bio).
		Where(sq.Eq{"telegram_id": telegramID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("error building query: %v", err)
	}

	_, err = r.db.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("error executing query: %v", err)
	}

	return nil
}

// Получаем внутренний ID пользователя по его telegram_id
func (r *UserRepository) GetUserIDByTelegramID(telegramID int64) (int, error) {
	query := r.builder.Select("id").
		From("users").
		Where(sq.Eq{"telegram_id": telegramID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("error building query: %v", err)
	}

	var userID int
	err = r.db.QueryRow(sqlQuery, args...).Scan(&userID)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return userID, nil
}

// Метод для добавления фото в таблицу photos
func (r *UserRepository) AddPhotoForUser(userID int, fileID string) error {
	query := r.builder.Insert("photos").
		Columns("user_id", "photo_url", "is_verified").
		Values(userID, fileID, false) // По умолчанию фото не верифицировано

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("error building query: %v", err)
	}

	_, err = r.db.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("error executing query: %v", err)
	}

	return nil
}

// Метод для получения фото пользователя
func (r *UserRepository) GetUserPhoto(telegramID int64) (string, error) {
	// Сначала получаем внутренний user_id по telegram_id
	var userID int
	query := r.builder.Select("id").
		From("users").
		Where(sq.Eq{"telegram_id": telegramID}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("error building query: %v", err)
	}

	err = r.db.QueryRow(sqlQuery, args...).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", nil // Пользователь не найден
	} else if err != nil {
		return "", err
	}

	// Теперь используем user_id для поиска фото
	query = r.builder.Select("photo_url").
		From("photos").
		Where(sq.Eq{"user_id": userID}).
		Limit(1)

	sqlQuery, args, err = query.ToSql()
	if err != nil {
		return "", fmt.Errorf("error building query for photos: %v", err)
	}

	var photoURL string
	err = r.db.QueryRow(sqlQuery, args...).Scan(&photoURL)
	if err == sql.ErrNoRows {
		return "", nil // Фото не найдено
	} else if err != nil {
		return "", err
	}

	return photoURL, nil
}

func (r *UserRepository) GetUserByID(userID string) (*User, error) {
	query := r.builder.Select("id", "telegram_id", "username", "first_name", "last_name", "gender", "age", "bio").
		From("users").
		Where(sq.Eq{"id": userID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building query: %v", err)
	}

	var user User
	err = r.db.QueryRow(sqlQuery, args...).Scan(&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName, &user.Gender, &user.Age, &user.Bio)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserPhotoByID(userID string) (string, error) {
	query := r.builder.Select("photo_url").
		From("photos").
		Where(sq.Eq{"user_id": userID}).
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("error building query for photos: %v", err)
	}

	var photoURL string
	err = r.db.QueryRow(sqlQuery, args...).Scan(&photoURL)
	if err == sql.ErrNoRows {
		return "", nil // Фото не найдено
	} else if err != nil {
		return "", err
	}

	return photoURL, nil
}

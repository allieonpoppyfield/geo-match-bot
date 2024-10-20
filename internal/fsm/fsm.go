package fsm

import (
	"geo_match_bot/internal/cache"
	"strconv"
)

// Определяем состояния
const (
	StepTitleName = "step_title_name"
	StepGender    = "step_gender"
	StepAge       = "step_age"
	StepBio       = "step_bio"
	StepPhoto     = "step_photo"
)

const (
	StepSearchGender             = "step_search_gender"
	StepSearchAge                = "step_search_age"
	StepSearchLocation           = "step_search_location"
	StepSetLocationForVisibility = "step_set_location_for_visibility"
)

type FSM struct {
	cache *cache.MemcacheClient
}

// Конструктор FSM
func NewFSM(cache *cache.MemcacheClient) *FSM {
	return &FSM{cache: cache}
}

// Установить текущее состояние для пользователя
func (fsm *FSM) SetState(userID int64, state string) error {
	return fsm.cache.Set(strconv.FormatInt(userID, 10), state)
}

// Получить текущее состояние пользователя
func (fsm *FSM) GetState(userID int64) (string, error) {
	return fsm.cache.Get(strconv.FormatInt(userID, 10))
}

// Очистить состояние пользователя
func (fsm *FSM) ClearState(userID int64) error {
	return fsm.cache.Delete(strconv.FormatInt(userID, 10))
}

// Логика перехода к следующему шагу
func (fsm *FSM) NextStep(userID int64) (string, error) {
	currentState, err := fsm.GetState(userID)
	if err != nil {
		return "", err
	}

	var nextState string
	switch currentState {
	case StepTitleName:
		nextState = StepGender
	case StepGender:
		nextState = StepAge
	case StepAge:
		nextState = StepBio
	case StepBio:
		nextState = StepPhoto
	default:
		nextState = ""
	}

	if nextState != "" {
		err := fsm.SetState(userID, nextState)
		if err != nil {
			return "", err
		}
	}

	return nextState, nil
}

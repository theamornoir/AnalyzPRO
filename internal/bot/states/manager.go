package states

import (
	"sync"
)

type State string

const (
	StateIdle                      State = "idle"
	StateWaitingAnalysisFile       State = "waiting_analysis_file"
	StateWaitingCourseInfo         State = "waiting_course_info" // Вопрос про препараты
	StateWaitingCourseTime         State = "waiting_course_time" // Какие препараты
	StateWaitingPhotoConfirm       State = "waiting_photo_confirm"
	StateWaitingFilesConfirm       State = "waiting_files_confirm"
	StateWaitingName               State = "waiting_name"
	StateWaitingGender             State = "waiting_gender"
	StateWaitingAge                State = "waiting_age"
	StateWaitingHeight             State = "waiting_height"
	StateWaitingWeight             State = "waiting_weight"
	StateWaitingChronicDiseases    State = "waiting_chronic_diseases"
	StateWaitingAllergies          State = "waiting_allergies"
	StateWaitingMedications        State = "waiting_medications"
	StateWaitingSmoking            State = "waiting_smoking"
	StateWaitingAlcohol            State = "waiting_alcohol"
	StateWaitingSportType          State = "waiting_sport_type"
	StateWaitingTrainingExperience State = "waiting_training_experience"
	StateWaitingGoal               State = "waiting_goal"

	StateWaitingBioscanName    State = "waiting_bioscan_name"
	StateWaitingBioscanAge     State = "waiting_bioscan_age"
	StateWaitingBioscanHeight  State = "waiting_bioscan_height"
	StateWaitingBioscanWeight  State = "waiting_bioscan_weight"
	StateWaitingBioscanGoal    State = "waiting_bioscan_goal"
	StateWaitingBioscanPhoto1  State = "waiting_bioscan_photo1"
	StateWaitingBioscanPhoto2  State = "waiting_bioscan_photo2"
	StateWaitingBioscanPhoto3  State = "waiting_bioscan_photo3"
	StateWaitingBioscanPhoto4  State = "waiting_bioscan_photo4"
	StateWaitingBioscanConfirm State = "waiting_bioscan_confirm"

	StateWaitingUploadConfirm State = "waiting_upload_confirm"
)

type StateManager interface {
	SetState(chatID int64, state State)
	GetState(chatID int64) State
	Reset(chatID int64)
	SetUserData(chatID int64, key, value string)
	GetUserData(chatID int64, key string) string
	GetAllUserData(chatID int64) map[string]string
}

type MemoryStateManager struct {
	mu     sync.RWMutex
	states map[int64]State
	data   map[int64]map[string]string
}

func NewMemoryStateManager() *MemoryStateManager {
	return &MemoryStateManager{
		states: make(map[int64]State),
		data:   make(map[int64]map[string]string),
	}
}

func (m *MemoryStateManager) SetState(chatID int64, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[chatID] = state
}

func (m *MemoryStateManager) GetState(chatID int64) State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if state, ok := m.states[chatID]; ok {
		return state
	}
	return StateIdle
}

func (m *MemoryStateManager) Reset(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, chatID)
	delete(m.data, chatID)
}

func (m *MemoryStateManager) SetUserData(chatID int64, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[chatID]; !ok {
		m.data[chatID] = make(map[string]string)
	}
	m.data[chatID][key] = value
}

func (m *MemoryStateManager) GetUserData(chatID int64, key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, ok := m.data[chatID]; ok {
		if val, ok := data[key]; ok {
			return val
		}
	}
	return ""
}

func (m *MemoryStateManager) GetAllUserData(chatID int64) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, ok := m.data[chatID]; ok {
		result := make(map[string]string)
		for k, v := range data {
			result[k] = v
		}
		return result
	}
	return make(map[string]string)
}

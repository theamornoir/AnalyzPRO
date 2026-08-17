package states

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type State string

const (
	StateIdle                State = "idle"
	StateWaitingAnalysisFile State = "waiting_analysis_file"

	StateWaitingName               State = "waiting_name"
	StateWaitingGender             State = "waiting_gender"
	StateWaitingAge                State = "waiting_age"
	StateWaitingHeight             State = "waiting_height"
	StateWaitingWeight             State = "waiting_weight"
	StateWaitingSleep              State = "waiting_sleep"               // Образ жизни: сон
	StateWaitingStress             State = "waiting_stress"              // Образ жизни: стресс
	StateWaitingNutritionVeg       State = "waiting_nutrition_veg"       // Овощи/фрукты
	StateWaitingNutritionProcessed State = "waiting_nutrition_processed" // Ультраобработанные
	StateWaitingWater              State = "waiting_water"               // Питьевой режим
	StateWaitingActivity           State = "waiting_activity"            // Физ. активность
	StateWaitingChronicDiseases    State = "waiting_chronic_diseases"
	StateWaitingAllergies          State = "waiting_allergies"
	StateWaitingMedications        State = "waiting_medications"
	StateWaitingSmoking            State = "waiting_smoking"
	StateWaitingAlcohol            State = "waiting_alcohol"
	StateWaitingFamilyHistory      State = "waiting_family_history" // Семейный анамнез
	StateWaitingDigestion          State = "waiting_digestion"      // ЖКТ / пищеварение
	StateWaitingSportType          State = "waiting_sport_type"
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

	// StateWaitingBioscanBasicPhoto — базовый (бесплатный) Bioscan: ожидание
	// одного фото пользователя (без вопросника и без PDF).
	StateWaitingBioscanBasicPhoto State = "waiting_bioscan_basic_photo"

	StateWaitingUploadConfirm State = "waiting_upload_confirm"

	// StateWaitingFeedback — режим ввода отзыва/предложения: следующее
	// сообщение пользователя (текст/фото/документ) пересылается админу.
	StateWaitingFeedback State = "waiting_feedback"

	// StateWaitingConsultation — режим «Быстрая консультация (с ИИ)»:
	// следующее сообщение пользователя (текстовый вопрос или фото травмы)
	// отправляется ИИ для генерации консультации с рекомендациями.
	StateWaitingConsultation State = "waiting_consultation"
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
	mu       sync.RWMutex
	filePath string
	states   map[int64]State
	data     map[int64]map[string]string
}

// persistedState - структура для сохранения состояния на диск
type persistedState struct {
	States map[int64]State             `json:"states"`
	Data   map[int64]map[string]string `json:"data"`
}

// NewMemoryStateManager создаёт in-memory менеджер состояний.
// Если filePath не пуст, состояния будут персистентно сохраняться на диск.
func NewMemoryStateManager(filePath string) *MemoryStateManager {
	m := &MemoryStateManager{
		filePath: filePath,
		states:   make(map[int64]State),
		data:     make(map[int64]map[string]string),
	}
	if filePath != "" {
		m.load()
	}
	return m
}

func (m *MemoryStateManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	file, err := os.Open(m.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	var ps persistedState
	if err := json.NewDecoder(file).Decode(&ps); err != nil {
		return
	}
	if ps.States != nil {
		m.states = ps.States
	}
	if ps.Data != nil {
		m.data = ps.Data
	}
}

func (m *MemoryStateManager) save() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.filePath == "" {
		return
	}

	if dir := filepath.Dir(m.filePath); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}

	file, err := os.Create(m.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	ps := persistedState{
		States: m.states,
		Data:   m.data,
	}
	_ = json.NewEncoder(file).Encode(ps)
}

func (m *MemoryStateManager) persist() {
	go m.save()
}

func (m *MemoryStateManager) SetState(chatID int64, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[chatID] = state
	m.persist()
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
	m.persist()
}

func (m *MemoryStateManager) SetUserData(chatID int64, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[chatID]; !ok {
		m.data[chatID] = make(map[string]string)
	}
	m.data[chatID][key] = value
	m.persist()
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

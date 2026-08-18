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

	StateWaitingBioscanName   State = "waiting_bioscan_name"
	StateWaitingBioscanAge    State = "waiting_bioscan_age"
	StateWaitingBioscanHeight State = "waiting_bioscan_height"
	StateWaitingBioscanWeight State = "waiting_bioscan_weight"
	StateWaitingBioscanGoal   State = "waiting_bioscan_goal"

	// Состояния опросника расширенного Bioscan PRO (отдельный от анализа
	// блок вопросов про образ жизни, спорт, травмы и здоровье). Они идут
	// ПОСЛЕ цели и ДО загрузки фотографий, чтобы отчёт Body Intelligence
	// учитывал не только фото, но и анкету пользователя.
	StateWaitingBioscanTrainingExp      State = "waiting_bioscan_training_exp"
	StateWaitingBioscanTrainingFreq     State = "waiting_bioscan_training_freq"
	StateWaitingBioscanTrainingType     State = "waiting_bioscan_training_type"
	StateWaitingBioscanInjuries         State = "waiting_bioscan_injuries"
	StateWaitingBioscanPostureIssues    State = "waiting_bioscan_posture_issues"
	StateWaitingBioscanImproveZones     State = "waiting_bioscan_improve_zones"
	StateWaitingBioscanMobility         State = "waiting_bioscan_mobility"
	StateWaitingBioscanRecovery         State = "waiting_bioscan_recovery"
	StateWaitingBioscanSleep            State = "waiting_bioscan_sleep"
	StateWaitingBioscanStress           State = "waiting_bioscan_stress"
	StateWaitingBioscanNutrition        State = "waiting_bioscan_nutrition"
	StateWaitingBioscanProtein          State = "waiting_bioscan_protein"
	StateWaitingBioscanWater            State = "waiting_bioscan_water"
	StateWaitingBioscanSmoking          State = "waiting_bioscan_smoking"
	StateWaitingBioscanAlcohol          State = "waiting_bioscan_alcohol"
	StateWaitingBioscanSedentary        State = "waiting_bioscan_sedentary"
	StateWaitingBioscanBodyFatGoal      State = "waiting_bioscan_body_fat_goal"
	StateWaitingBioscanDietRestrictions State = "waiting_bioscan_diet_restrictions"

	StateWaitingBioscanPhoto1  State = "waiting_bioscan_photo1"
	StateWaitingBioscanPhoto2  State = "waiting_bioscan_photo2"
	StateWaitingBioscanPhoto3  State = "waiting_bioscan_photo3"
	StateWaitingBioscanPhoto4  State = "waiting_bioscan_photo4"
	StateWaitingBioscanConfirm State = "waiting_bioscan_confirm"

	// StateWaitingBioscanBasicPhoto - базовый (бесплатный) Bioscan: ожидание
	// одного фото пользователя (без вопросника и без PDF).
	StateWaitingBioscanBasicPhoto State = "waiting_bioscan_basic_photo"

	StateWaitingUploadConfirm State = "waiting_upload_confirm"

	// StateWaitingFeedback - режим ввода отзыва/предложения: следующее
	// сообщение пользователя (текст/фото/документ) пересылается админу.
	StateWaitingFeedback State = "waiting_feedback"

	// StateWaitingConsultation - режим «Быстрая консультация (с ИИ)»:
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
	// Premium-экран: отдельные методы трекинга id сообщений экрана Premium
	// (якорь + список/оплата/подтверждение). Намеренно ВЫНЕСЕНЫ из общего
	// user-data (m.data), потому что stateManager.Reset(chatID) очищает
	// m.data целиком - иначе id экрана Premium терялись бы при /start и
	// сообщение навсегда «висело» в чате. Здесь же запись синхронная
	// (без go), чтобы id гарантированно попал на диск ДО возможного
	// перезапуска бота (асинхронный go m.save() мог не успеть при kill).
	SetPremiumScreenID(chatID int64, key, value string)
	GetPremiumScreenID(chatID int64, key string) string
	ClearPremiumScreenIDs(chatID int64)
}

type MemoryStateManager struct {
	mu       sync.RWMutex
	filePath string
	states   map[int64]State
	data     map[int64]map[string]string
	// premiumScreen - отдельное хранилище id сообщений экрана Premium.
	// Не очищается Reset (в отличие от m.data), персистится и пишется
	// синхронно - см. обоснование в интерфейсе StateManager.
	premiumScreen map[int64]map[string]string
}

// persistedState - структура для сохранения состояния на диск
type persistedState struct {
	States  map[int64]State             `json:"states"`
	Data    map[int64]map[string]string `json:"data"`
	Premium map[int64]map[string]string `json:"premium"`
}

// NewMemoryStateManager создаёт in-memory менеджер состояний.
// Если filePath не пуст, состояния будут персистентно сохраняться на диск.
func NewMemoryStateManager(filePath string) *MemoryStateManager {
	m := &MemoryStateManager{
		filePath:      filePath,
		states:        make(map[int64]State),
		data:          make(map[int64]map[string]string),
		premiumScreen: make(map[int64]map[string]string),
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
	if ps.Premium != nil {
		m.premiumScreen = ps.Premium
	}
}

func (m *MemoryStateManager) save() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.saveLocked()
}

// saveLocked - записывает состояние на диск. ВЫЗЫВАТЬ ТОЛЬКО УЖЕ ДЕРЖА
// m.mu (под RLock или Lock) - сама блокировку НЕ берёт (иначе deadlock при
// вызове из-под Lock в SetPremiumScreenID/ClearPremiumScreenIDs).
func (m *MemoryStateManager) saveLocked() {
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
		States:  m.states,
		Data:    m.data,
		Premium: m.premiumScreen,
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

// --- Трекинг id сообщений экрана Premium (якорь + список/оплата/
// подтверждение). Хранится в ОТДЕЛЬНОМ map (premiumScreen), который НЕ
// очищается Reset, и пишется СИНХРОННО (без go), чтобы id экрана
// гарантированно попал на диск до перезапуска бота. Это устраняет
// «висящий» экран Premium, который ранее не удалялся после /start или
// перезапуска, потому что его id терялся (Reset стирал m.data, а
// асинхронный save мог не успеть при kill).

func (m *MemoryStateManager) SetPremiumScreenID(chatID int64, key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.premiumScreen[chatID]; !ok {
		m.premiumScreen[chatID] = make(map[string]string)
	}
	m.premiumScreen[chatID][key] = value
	// Синхронно - id критично сохранить до возможного перезапуска.
	m.saveLocked()
}

func (m *MemoryStateManager) GetPremiumScreenID(chatID int64, key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.premiumScreen[chatID]; ok {
		if v, ok := d[key]; ok {
			return v
		}
	}
	return ""
}

func (m *MemoryStateManager) ClearPremiumScreenIDs(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.premiumScreen, chatID)
	m.saveLocked()
}

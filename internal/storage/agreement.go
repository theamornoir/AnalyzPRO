package storage

import (
	"encoding/json"
	"os"
	"sync"
)

type AgreementStorage struct {
	mu       sync.RWMutex
	filePath string
	data     map[int64]bool // chatID -> agreed
}

func NewAgreementStorage(filePath string) *AgreementStorage {
	s := &AgreementStorage{
		filePath: filePath,
		data:     make(map[int64]bool),
	}
	s.load()
	return s
}

func (s *AgreementStorage) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		return
	}
	defer file.Close()

	json.NewDecoder(file).Decode(&s.data)
}

func (s *AgreementStorage) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Create(s.filePath)
	if err != nil {
		return
	}
	defer file.Close()

	json.NewEncoder(file).Encode(s.data)
}

func (s *AgreementStorage) SetAgreed(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[chatID] = true
	go s.save()
}

func (s *AgreementStorage) IsAgreed(chatID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[chatID]
}

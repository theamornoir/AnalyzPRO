package storage

import (
	"context"
	"time"

	sm "github.com/theamornoir/analyzpro/internal/storage/models"
)

// EnsureUser возвращает пользователя по TelegramID, создавая его (и
// дефолтные предпочтения) при отсутствии. Используется на онбординге и
// перед сохранением результата анализа/биоскана, чтобы Diagnosis всегда
// был привязан к существующему User.
func (s *Storage) EnsureUser(ctx context.Context, telegramID int64) (*sm.User, error) {
	if u, err := s.Users.GetUserByTelegramID(ctx, telegramID); err == nil {
		return u, nil
	}

	u := &sm.User{TelegramID: telegramID}
	if err := s.Users.CreateUser(ctx, u); err != nil {
		return nil, err
	}

	created, err := s.Users.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		// По крайней мере сохранили — возвращаем переданный объект.
		return u, nil
	}

	// Дефолтные предпочтения (идемпотентно).
	if _, perr := s.Preferences.GetPreferences(ctx, created.ID); perr != nil {
		_ = s.Preferences.UpdatePreferences(ctx, &sm.Preference{
			UserID:               created.ID,
			ReminderFrequency:    "daily",
			Units:                "metric",
			NotificationsEnabled: true,
		})
	}
	return created, nil
}

// SaveDiagnosisForUser сохраняет результат анализа/биоскана, привязывая
// к пользователю по TelegramID (создаёт пользователя при необходимости).
func (s *Storage) SaveDiagnosisForUser(
	ctx context.Context,
	telegramID int64,
	diagnosisType, jsonData, reportHTML string,
) error {
	u, err := s.EnsureUser(ctx, telegramID)
	if err != nil {
		return err
	}
	return s.Diagnoses.SaveDiagnosis(ctx, &sm.Diagnosis{
		UserID:     u.ID,
		Date:       time.Now(),
		Type:       diagnosisType,
		JsonData:   jsonData,
		ReportHTML: reportHTML,
	})
}

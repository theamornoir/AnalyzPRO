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
		// По крайней мере сохранили - возвращаем переданный объект.
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

// SetOnboardingCompleted устанавливает (и создаёт при необходимости)
// пользователя с флагом прохождения онбординга по TelegramID.
func (s *Storage) SetOnboardingCompleted(ctx context.Context, telegramID int64, completed bool) error {
	u, err := s.EnsureUser(ctx, telegramID)
	if err != nil {
		return err
	}
	return s.Users.UpdateUserOnboardingStatus(ctx, u.ID, completed)
}

// IsOnboardingCompleted возвращает true, если пользователь уже прошёл
// онбординг. Для несуществующего пользователя - false.
func (s *Storage) IsOnboardingCompleted(ctx context.Context, telegramID int64) bool {
	u, err := s.Users.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return false
	}
	return u.OnboardingCompleted
}

// TouchActivity обновляет дату последнего взаимодействия пользователя с
// ботом. Используется системой уведомлений, чтобы определять период
// неактивности и напоминать о повторном анализе. Пользователь создаётся
// при необходимости (аналогично EnsureUser).
func (s *Storage) TouchActivity(ctx context.Context, telegramID int64) error {
	u, err := s.Users.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		u, err = s.EnsureUser(ctx, telegramID)
		if err != nil {
			return err
		}
	}
	return s.Users.UpdateUserLastActivity(ctx, u.ID, time.Now())
}

// GetAllUsers возвращает всех пользователей (для периодических рассылок).
func (s *Storage) GetAllUsers(ctx context.Context) ([]*sm.User, error) {
	return s.Users.GetAllUsers(ctx)
}

// DeleteAccount полностью удаляет аккаунт пользователя по TelegramID:
// профиль, анализы, курсы, предпочтения. Данные мониторинга и уведомлений
// удаляются соответствующими сервисами (monitorRepo / notifications).
// Необратимо. Если пользователя нет - молча завершается.
func (s *Storage) DeleteAccount(ctx context.Context, telegramID int64) error {
	return s.Users.DeleteAccount(ctx, telegramID)
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

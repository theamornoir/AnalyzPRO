package sqlrepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	sm "github.com/theamornoir/analyzpro/internal/storage/models"
	"github.com/theamornoir/analyzpro/internal/storage/sqlrepo"
)

func newTestStorage(t *testing.T) *sqlrepo.Repo {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return sqlrepo.New(conn)
}

func TestUserRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newTestStorage(t)

	created := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := r.CreateUser(ctx, &sm.User{TelegramID: 123, Name: "Алексей", CreatedAt: created}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	u, err := r.GetUserByTelegramID(ctx, 123)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.Name != "Алексей" {
		t.Errorf("name = %q, want Алексей", u.Name)
	}
	if u.ID == 0 {
		t.Error("expected non-zero ID")
	}
	// CreatedAt должен корректно сканироваться из БД.
	if u.CreatedAt.IsZero() {
		t.Error("CreatedAt не восстановлен из БД")
	}

	// Upsert по тому же telegram_id не должен создавать дубль.
	if err := r.CreateUser(ctx, &sm.User{TelegramID: 123, Name: "Обновлён"}); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	u2, _ := r.GetUserByTelegramID(ctx, 123)
	if u2.Name != "Обновлён" {
		t.Errorf("upsert name = %q, want Обновлён", u2.Name)
	}
	if u2.ID != u.ID {
		t.Error("upsert изменил ID пользователя")
	}

	if err := r.UpdateUserPremiumStatus(ctx, u.ID, true, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("update premium: %v", err)
	}
	u3, _ := r.GetUserByTelegramID(ctx, 123)
	if !u3.IsPremium {
		t.Error("premium не активирован")
	}
}

func TestDiagnosisRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := newTestStorage(t)
	_ = r.CreateUser(ctx, &sm.User{TelegramID: 1})

	d := &sm.Diagnosis{UserID: 1, Date: time.Now(), Type: "analysis", JsonData: `{"a":1}`, ReportHTML: "<b>x</b>"}
	if err := r.SaveDiagnosis(ctx, d); err != nil {
		t.Fatalf("save diagnosis: %v", err)
	}
	if d.ID == 0 {
		t.Error("expected non-zero diagnosis ID")
	}

	all, err := r.GetAllDiagnosesByUserID(ctx, 1)
	if err != nil || len(all) != 1 {
		t.Fatalf("get all: %v (len=%d)", err, len(all))
	}

	last, err := r.GetLastDiagnosisByType(ctx, 1, "analysis")
	if err != nil {
		t.Fatalf("get last: %v", err)
	}
	if last.JsonData != `{"a":1}` {
		t.Errorf("json mismatch: %q", last.JsonData)
	}
}

func TestCycleAndPreferences(t *testing.T) {
	ctx := context.Background()
	r := newTestStorage(t)
	_ = r.CreateUser(ctx, &sm.User{TelegramID: 5})

	start := time.Now().Add(-time.Hour)
	if err := r.CreateCycle(ctx, &sm.Cycle{UserID: 5, Name: "Курс", StartDate: start, TrackedMarkers: []string{"Hb", "Глюкоза"}}); err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	active, err := r.GetActiveCycleByUserID(ctx, 5)
	if err != nil {
		t.Fatalf("get active cycle: %v", err)
	}
	if active.Name != "Курс" {
		t.Errorf("cycle name = %q", active.Name)
	}
	if len(active.TrackedMarkers) != 2 {
		t.Errorf("tracked markers не восстановлены: %v", active.TrackedMarkers)
	}

	if err := r.CompleteCycle(ctx, active.ID, time.Now()); err != nil {
		t.Fatalf("complete cycle: %v", err)
	}
	if _, err := r.GetActiveCycleByUserID(ctx, 5); err == nil {
		t.Error("ожидали отсутствие активного курса после завершения")
	}

	if err := r.UpdatePreferences(ctx, &sm.Preference{UserID: 5, ReminderFrequency: "weekly", Units: "metric", NotificationsEnabled: false}); err != nil {
		t.Fatalf("update prefs: %v", err)
	}
	p, err := r.GetPreferences(ctx, 5)
	if err != nil {
		t.Fatalf("get prefs: %v", err)
	}
	if p.ReminderFrequency != "weekly" || p.NotificationsEnabled {
		t.Errorf("prefs mismatch: %+v", p)
	}
}

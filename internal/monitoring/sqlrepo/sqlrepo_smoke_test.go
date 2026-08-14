package sqlrepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/theamornoir/analyzpro/internal/db"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/monitoring/sqlrepo"
)

func newTestRepo(t *testing.T) monitoring.Repository {
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

func TestMonitoringHistoryAndProjects(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)

	// История
	if err := repo.SaveResult(ctx, &monitoring.HistoryEntry{
		TelegramID: 42, Type: "analysis", Title: "A1",
		Date: time.Now().Add(-48 * time.Hour), JsonData: `{"markers":[{"name":"Глюкоза","value":6.1}]}`,
	}); err != nil {
		t.Fatalf("save history: %v", err)
	}
	if err := repo.SaveResult(ctx, &monitoring.HistoryEntry{
		TelegramID: 42, Type: "analysis", Title: "A2",
		Date: time.Now(), JsonData: `{"markers":[{"name":"Глюкоза","value":5.2}]}`,
	}); err != nil {
		t.Fatalf("save history: %v", err)
	}

	hist, total, err := repo.ListHistory(ctx, 42, "", 1, 50)
	if err != nil {
		t.Fatalf("list history: %v", err)
	}
	if total != 2 || len(hist) != 2 {
		t.Fatalf("expected 2 entries, got total=%d len=%d", total, len(hist))
	}
	if !hist[0].Date.After(hist[1].Date) {
		t.Error("история не отсортирована по убыванию даты")
	}

	if err := repo.CreateProject(ctx, &monitoring.MonitoringProject{
		TelegramID: 42, Name: "Курс", Type: "course", StartDate: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	projects, err := repo.ListProjects(ctx, 42)
	if err != nil || len(projects) == 0 {
		t.Fatalf("list projects: %v (len=%d)", err, len(projects))
	}
	p := projects[0]

	if err := repo.BindEntry(ctx, p.ID, hist[0].ID); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := repo.BindEntry(ctx, p.ID, hist[1].ID); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := repo.BindEntry(ctx, p.ID, 999999); err == nil {
		t.Error("ожидали ошибку привязки несуществующей записи")
	}

	entries, err := repo.ListProjectEntries(ctx, p.ID)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("ожидали 2 привязки, получили %d", len(entries))
	}

	proj, err := repo.GetProject(ctx, p.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if len(proj.EntryIDs) != 2 {
		t.Errorf("project EntryIDs = %v, want 2", proj.EntryIDs)
	}

	if err := repo.CompleteProject(ctx, p.ID, time.Now()); err != nil {
		t.Fatalf("complete: %v", err)
	}
	proj, _ = repo.GetProject(ctx, p.ID)
	if proj.Status != monitoring.ProjectStatusCompleted {
		t.Errorf("status = %q, want completed", proj.Status)
	}
}

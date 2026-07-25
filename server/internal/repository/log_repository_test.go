package repository

import (
	"testing"
	"time"

	"v2ray-server/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLogRepositoryForTest(t *testing.T) *LogRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("migrate log model: %v", err)
	}
	return NewLogRepository(db)
}

func seedLog(t *testing.T, repo *LogRepository, msg string) {
	t.Helper()
	err := repo.Create(&model.Log{
		Tag:       "test",
		Message:   msg,
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("seed log: %v", err)
	}
}

func TestLogRepositoryDeleteAll(t *testing.T) {
	repo := newLogRepositoryForTest(t)
	seedLog(t, repo, "first")
	seedLog(t, repo, "second")

	deleted, err := repo.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll returned error: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	logs, _, err := repo.FindByFilter(model.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("FindByFilter returned error: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected empty log list, got %d entries", len(logs))
	}
}

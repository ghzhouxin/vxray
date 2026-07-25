package service

import (
	"testing"
	"time"

	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newLogServiceForTest(t *testing.T) *LogService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Log{}); err != nil {
		t.Fatalf("migrate log model: %v", err)
	}
	return &LogService{repo: repository.NewLogRepository(db)}
}

func TestLogServiceClearDeletesAll(t *testing.T) {
	svc := newLogServiceForTest(t)

	if err := svc.Create("test", "first", nil); err != nil {
		t.Fatalf("create first log: %v", err)
	}
	time.Sleep(1 * time.Millisecond)
	if err := svc.Create("test", "second", map[string]any{"ok": true}); err != nil {
		t.Fatalf("create second log: %v", err)
	}

	deleted, err := svc.Clear()
	if err != nil {
		t.Fatalf("clear logs: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	logs, _, err := svc.List(model.LogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list logs after clear: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected empty list after clear, got %d", len(logs))
	}
}

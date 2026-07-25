package service

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"

	"gorm.io/gorm"
)

type LogService struct {
	repo *repository.LogRepository
}

type OperationLog struct {
	ID   string
	logs *LogService
}

var operationSeq uint64

func NewLogService(db *gorm.DB) *LogService {
	return &LogService{repo: repository.NewLogRepository(db)}
}

func (s *LogService) Create(tag, message string, detail any) error {
	return s.createEntry("", tag, message, detail)
}

func marshalDetail(detail any) string {
	if detail == nil {
		return ""
	}
	data, err := json.Marshal(detail)
	if err != nil {
		return fmt.Sprintf("%v", detail)
	}
	return string(data)
}

func (s *LogService) createEntry(operationID, tag, message string, detail any) error {
	now := time.Now()
	log := model.Log{
		OperationID: operationID,
		Tag:         tag,
		Message:     message,
		Detail:      marshalDetail(detail),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.repo.Create(&log)
}

func (s *LogService) LogLevel(tag, level, message string, detail any) error {
	return s.Create(tag, message, mergeLogDetail(level, detail))
}

func (s *LogService) Info(tag, message string, detail any) error {
	return s.LogLevel(tag, "info", message, detail)
}

func (s *LogService) Error(tag, message string, detail any) error {
	return s.LogLevel(tag, "error", message, detail)
}

type TaggedLogger struct {
	logs *LogService
	tag  string
}

func (s *LogService) NewTaggedLogger(tag string) *TaggedLogger {
	return &TaggedLogger{logs: s, tag: tag}
}

func (l *TaggedLogger) Info(message string, detail map[string]any) {
	if l.logs != nil {
		_ = l.logs.Info(l.tag, message, detail)
	}
}

func (l *TaggedLogger) Error(message string, detail map[string]any) {
	if l.logs != nil {
		_ = l.logs.Error(l.tag, message, detail)
	}
}

func (s *LogService) StartOperation(tag, message string, detail any) (*OperationLog, error) {
	operationID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&operationSeq, 1))
	if err := s.createEntry(operationID, tag, message, mergeLogDetail("info", detail)); err != nil {
		return nil, err
	}
	return &OperationLog{ID: operationID, logs: s}, nil
}

func (s *LogService) RunOperation(tag, message string, detail any, fn func() error) error {
	op, opErr := s.StartOperation(tag, message, detail)
	if opErr != nil {
		log.Printf("LogService: 启动操作日志失败 tag=%s message=%s err=%v", tag, message, opErr)
	}
	err := fn()
	if err != nil {
		if op != nil {
			_ = op.Fail(message+"失败", map[string]any{"error": err.Error()})
		}
		return err
	}
	if op != nil {
		_ = op.Success(message+"完成", nil)
	}
	return nil
}

func (s *LogService) updateOperation(operationID, message string, detail any) error {
	updates := map[string]any{
		"message":    message,
		"updated_at": time.Now(),
	}
	if detail != nil {
		updates["detail"] = marshalDetail(detail)
	}
	return s.repo.UpdateOperation(operationID, updates)
}

func (o *OperationLog) Update(message string, detail any) error {
	if o == nil || o.logs == nil || o.ID == "" {
		return nil
	}
	return o.logs.updateOperation(o.ID, message, mergeLogDetail("info", detail))
}

func (o *OperationLog) Success(message string, detail any) error {
	return o.Update(message, detail)
}

func (o *OperationLog) Fail(message string, detail any) error {
	if o == nil || o.logs == nil || o.ID == "" {
		return nil
	}
	return o.logs.updateOperation(o.ID, message, mergeLogDetail("error", detail))
}

func (s *LogService) List(filter model.LogFilter) ([]model.Log, string, error) {
	return s.repo.FindByFilter(filter)
}

func (s *LogService) Clear() (int64, error) {
	return s.repo.DeleteAll()
}

func (s *LogService) GetTags() []string {
	return constants.LogTags
}

func (s *LogService) GetLevels() []string {
	return constants.LogLevels
}

func mergeLogDetail(level string, detail any) map[string]any {
	merged := map[string]any{"level": level}
	switch value := detail.(type) {
	case nil:
		return merged
	case map[string]any:
		for k, v := range value {
			merged[k] = v
		}
	case map[string]string:
		for k, v := range value {
			merged[k] = v
		}
	default:
		merged["data"] = value
	}
	return merged
}

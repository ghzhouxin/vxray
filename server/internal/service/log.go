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
	ID     string
	logSvc *LogService
}

var operationSeq uint64

func NewLogService(db *gorm.DB) *LogService {
	return &LogService{repo: repository.NewLogRepository(db)}
}

// marshalDetail 序列化 detail 为 JSON 字符串；nil 返回空串。
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

// create 写入一条日志。level 为 debug/info/warn/error。
func (s *LogService) create(operationID, tag, level, message string, detail any) error {
	now := time.Now()
	entry := model.Log{
		OperationID: operationID,
		Level:       level,
		Tag:         tag,
		Message:     message,
		Detail:      marshalDetail(detail),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.repo.Create(&entry)
}

// LogLevel 写入指定级别的日志。xray 进程日志回调走这里。
func (s *LogService) LogLevel(tag, level, message string, detail any) error {
	return s.create("", tag, level, message, detail)
}

func (s *LogService) Info(tag, message string, detail any) error {
	return s.LogLevel(tag, constants.LevelInfo, message, detail)
}

func (s *LogService) Error(tag, message string, detail any) error {
	return s.LogLevel(tag, constants.LevelError, message, detail)
}

// --- Tagged logger ---

type TaggedLogger struct {
	logSvc *LogService
	tag    string
}

func (s *LogService) NewTaggedLogger(tag string) *TaggedLogger {
	return &TaggedLogger{logSvc: s, tag: tag}
}

func (l *TaggedLogger) Info(message string, detail any) {
	if l.logSvc != nil {
		_ = l.logSvc.Info(l.tag, message, detail)
	}
}

func (l *TaggedLogger) Error(message string, detail any) {
	if l.logSvc != nil {
		_ = l.logSvc.Error(l.tag, message, detail)
	}
}

// --- Operations（长任务单行可变记录，operation_id 索引但不暴露前端）---

func (s *LogService) StartOperation(tag, message string, detail any) (*OperationLog, error) {
	operationID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddUint64(&operationSeq, 1))
	if err := s.create(operationID, tag, constants.LevelInfo, message, detail); err != nil {
		return nil, err
	}
	return &OperationLog{ID: operationID, logSvc: s}, nil
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

func (s *LogService) updateOperation(operationID, level, message string, detail any) error {
	updates := map[string]any{
		"level":      level,
		"message":    message,
		"updated_at": time.Now(),
	}
	if detail != nil {
		updates["detail"] = marshalDetail(detail)
	}
	return s.repo.UpdateOperation(operationID, updates)
}

func (o *OperationLog) Update(message string, detail any) error {
	if o == nil {
		return nil
	}
	return o.logSvc.updateOperation(o.ID, constants.LevelInfo, message, detail)
}

func (o *OperationLog) Success(message string, detail any) error {
	if o == nil {
		return nil
	}
	return o.logSvc.updateOperation(o.ID, constants.LevelInfo, message, detail)
}

func (o *OperationLog) Fail(message string, detail any) error {
	if o == nil {
		return nil
	}
	return o.logSvc.updateOperation(o.ID, constants.LevelError, message, detail)
}

// --- Read ---

func (s *LogService) List(filter model.LogFilter) ([]model.Log, string, error) {
	return s.repo.FindByFilter(filter)
}

func (s *LogService) Clear() (int64, error) {
	return s.repo.DeleteAll()
}

func (s *LogService) GetTags() []string   { return constants.LogTags }
func (s *LogService) GetLevels() []string { return constants.LogLevels }

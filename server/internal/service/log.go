package service

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"

	"gorm.io/gorm"
)

type LogService struct {
	repo         *repository.LogRepository
	logFilePaths []string // vxray 托管的日志文件，Clear 时一并清空
}

func NewLogService(db *gorm.DB, logFilePaths ...string) *LogService {
	return &LogService{repo: repository.NewLogRepository(db), logFilePaths: logFilePaths}
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
func (s *LogService) create(tag, level, message string, detail any) error {
	now := time.Now()
	entry := model.Log{
		Level:     level,
		Tag:       tag,
		Message:   message,
		Detail:    marshalDetail(detail),
		CreatedAt: now,
		UpdatedAt: now,
	}
	return s.repo.Create(&entry)
}

// LogLevel 写入指定级别的日志。xray 进程日志回调走这里。
func (s *LogService) LogLevel(tag, level, message string, detail any) error {
	return s.create(tag, level, message, detail)
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
	_ = l.logSvc.Info(l.tag, message, detail)
}

func (l *TaggedLogger) Error(message string, detail any) {
	_ = l.logSvc.Error(l.tag, message, detail)
}

// --- Read ---

func (s *LogService) List(filter model.LogFilter) ([]model.Log, string, error) {
	return s.repo.FindByFilter(filter)
}

// Clear 全盘清理：删除 DB 日志并清空托管的日志文件。
// 日志文件以追加模式写入，truncate 不会产生稀疏空洞。
func (s *LogService) Clear() (int64, error) {
	for _, path := range s.logFilePaths {
		_ = os.Truncate(path, 0)
	}
	return s.repo.DeleteAll()
}

func (s *LogService) GetTags() []string   { return constants.LogTags }
func (s *LogService) GetLevels() []string { return constants.LogLevels }

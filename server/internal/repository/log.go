package repository

import (
	"strings"
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"

	"gorm.io/gorm"
)

type logCursor struct {
	UpdatedAt int64 `json:"updated_at"`
	ID        uint  `json:"id"`
}

type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) *LogRepository { return &LogRepository{db: db} }

func (r *LogRepository) Create(log *model.Log) error { return r.db.Create(log).Error }

func (r *LogRepository) UpdateOperation(operationID string, updates map[string]any) error {
	return r.db.Model(&model.Log{}).Where("operation_id = ?", operationID).Updates(updates).Error
}

func (r *LogRepository) FindByFilter(filter model.LogFilter) ([]model.Log, string, error) {
	var logs []model.Log
	query := r.db.Model(&model.Log{})
	if filter.Level != "" {
		query = query.Where("json_valid(detail) AND json_extract(detail, '$.level') = ?", filter.Level)
	}
	if filter.Tag != "" {
		query = query.Where("tag = ?", filter.Tag)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if filter.Keyword != "" {
		keyword := strings.TrimSpace(filter.Keyword)
		if keyword != "" {
			query = query.Where("message LIKE ? OR detail LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	if cursor, ok := decodeLogCursor(filter.Cursor); ok {
		updatedAt := time.Unix(0, cursor.UpdatedAt)
		query = query.Where("updated_at < ? OR (updated_at = ? AND id < ?)", updatedAt, updatedAt, cursor.ID)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = constants.DefaultLogPageSize
	}

	if err := query.Order("updated_at DESC").Order("id DESC").Limit(limit + 1).Find(&logs).Error; err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(logs) > limit {
		nextCursor = encodeLogCursor(logs[limit-1])
		logs = logs[:limit]
	}
	return logs, nextCursor, nil
}

func (r *LogRepository) DeleteAll() (int64, error) {
	result := r.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model.Log{})
	return result.RowsAffected, result.Error
}

func encodeLogCursor(log model.Log) string {
	return encodeCursor(logCursor{UpdatedAt: log.UpdatedAt.UnixNano(), ID: log.ID})
}

func decodeLogCursor(value string) (logCursor, bool) {
	var cursor logCursor
	return cursor, decodeCursor(value, &cursor)
}

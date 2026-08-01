package dto

import "time"

type LogFilter struct {
	Level     string `form:"level"`
	Tag       string `form:"tag"`
	Keyword   string `form:"keyword"`
	Limit     int    `form:"limit"`
	Cursor    string `form:"cursor"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

type LogDTO struct {
	ID        uint      `json:"id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Tag       string    `json:"tag"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LogListResponse struct {
	Items      []LogDTO `json:"items"`
	Tags       []string `json:"tags"`
	Levels     []string `json:"levels"`
	NextCursor string   `json:"next_cursor"`
	HasMore    bool     `json:"has_more"`
}

package repository

import (
	"encoding/base64"
	"encoding/json"
)

// encodeCursor 将任意结构体编码为 base64+JSON 游标字符串。
func encodeCursor(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// decodeCursor 将游标字符串解码到 v，失败返回 false。
func decodeCursor(value string, v any) bool {
	if value == "" {
		return false
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false
	}
	return true
}

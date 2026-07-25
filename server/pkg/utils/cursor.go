package utils

import (
	"encoding/base64"
	"encoding/json"
)

// EncodeCursor 将任意结构体编码为 base64+JSON 游标字符串。
func EncodeCursor(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func DecodeCursor(value string, v any) bool {
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

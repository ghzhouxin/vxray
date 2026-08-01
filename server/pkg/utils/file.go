package utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var errDecodeFailed = errors.New("base64 decode failed")

func DecodeBase64(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	encoded = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, encoded)

	decoders := []struct {
		name string
		enc  *base64.Encoding
	}{
		{"std", base64.StdEncoding},
		{"url", base64.URLEncoding},
		{"rawstd", base64.RawStdEncoding},
		{"rawurl", base64.RawURLEncoding},
	}

	for _, d := range decoders {
		result, err := d.enc.DecodeString(encoded)
		if err == nil {
			return result, nil
		}
	}

	return nil, errDecodeFailed
}

func EnsureDirs(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func EnsureFile(path string, defaultContent []byte) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, defaultContent, 0644)
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, data)
}

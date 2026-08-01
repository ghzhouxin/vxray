package dto

// TunStatusResponse 是 GET /api/tun/status 的响应。
type TunStatusResponse struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
}

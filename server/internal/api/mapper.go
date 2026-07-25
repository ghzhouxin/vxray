package api

import (
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
)

func toSubscriptionDTO(m model.Subscription) dto.SubscriptionDTO {
	return dto.SubscriptionDTO{
		ID:             m.ID,
		Name:           m.Name,
		URL:            m.URL,
		LastSyncAt:     m.LastSyncAt,
		LastSyncStatus: m.LastSyncStatus,
		NodeCount:      m.NodeCount,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func toSubscriptionDTOs(items []model.Subscription) []dto.SubscriptionDTO {
	result := make([]dto.SubscriptionDTO, len(items))
	for i, item := range items {
		result[i] = toSubscriptionDTO(item)
	}
	return result
}

func toLogDTO(m model.Log) dto.LogDTO {
	return dto.LogDTO{
		ID:        m.ID,
		Message:   m.Message,
		Tag:       m.Tag,
		Detail:    m.Detail,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func toLogDTOs(items []model.Log) []dto.LogDTO {
	result := make([]dto.LogDTO, len(items))
	for i, item := range items {
		result[i] = toLogDTO(item)
	}
	return result
}

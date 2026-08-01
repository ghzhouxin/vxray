package repository

import (
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"

	"gorm.io/gorm"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) FindAllWithNodeCount() ([]model.Subscription, error) {
	var subs []model.Subscription
	err := r.db.Model(&model.Subscription{}).
		Select("subscriptions.id, subscriptions.name, subscriptions.url, subscriptions.content_hash, subscriptions.last_sync_at, subscriptions.last_sync_status, subscriptions.created_at, subscriptions.updated_at, COUNT(nodes.id) as node_count").
		Joins("LEFT JOIN nodes ON subscriptions.id = nodes.subscription_id").
		Group("subscriptions.id").
		Scan(&subs).Error
	if subs == nil {
		subs = []model.Subscription{}
	}
	return subs, err
}

func (r *SubscriptionRepository) FindByIDWithNodeCount(id uint) (*model.Subscription, error) {
	var sub model.Subscription
	err := r.db.Model(&model.Subscription{}).
		Select("subscriptions.*, COUNT(nodes.id) as node_count").
		Joins("LEFT JOIN nodes ON subscriptions.id = nodes.subscription_id").
		Where("subscriptions.id = ?", id).
		Group("subscriptions.id").
		Scan(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepository) Create(sub *model.Subscription) error { return r.db.Create(sub).Error }
func (r *SubscriptionRepository) Update(sub *model.Subscription) error {
	return r.db.Model(&model.Subscription{}).Where("id = ?", sub.ID).Updates(map[string]any{
		"name": sub.Name,
		"url":  sub.URL,
	}).Error
}

func (r *SubscriptionRepository) DeleteWithNodes(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subscription_id = ?", id).Delete(&model.Node{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Subscription{}, id).Error
	})
}

// SaveNodesAndContentHash 原子地批量写入新节点并更新订阅 content_hash。
func (r *SubscriptionRepository) SaveNodesAndContentHash(nodes []*model.Node, subID uint, contentHash string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(nodes) > 0 {
			for _, n := range nodes {
				n.LatencyRank = latencyRank(n.Latency)
			}
			if err := tx.CreateInBatches(nodes, constants.NodeBatchSize).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.Subscription{}).Where("id = ?", subID).Update("content_hash", contentHash).Error
	})
}

func (r *SubscriptionRepository) UpdateSyncStatus(id uint, status string, syncedAt *time.Time) error {
	updates := map[string]any{
		"last_sync_status": status,
	}
	if syncedAt != nil {
		updates["last_sync_at"] = *syncedAt
	}
	return r.db.Model(&model.Subscription{}).Where("id = ?", id).Updates(updates).Error
}

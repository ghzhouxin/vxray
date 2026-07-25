package repository

import (
	"fmt"
	"strconv"
	"strings"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"

	"gorm.io/gorm"
)

type NodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) *NodeRepository { return &NodeRepository{db: db} }

type nodeCursor struct {
	ID          uint  `json:"id"`
	Latency     int64 `json:"latency,omitempty"`
	LatencyRank int   `json:"latency_rank,omitempty"`
}

func (r *NodeRepository) FindByID(id uint) (*model.Node, error) {
	var node model.Node
	if err := r.db.First(&node, id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *NodeRepository) FindByFilter(filter model.NodeFilter) ([]*model.Node, string, error) {
	var nodes []*model.Node
	query := r.buildFilteredQuery(filter)
	query = r.applyNodeCursor(query, filter.Cursor)
	query = r.applyNodeSort(query)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit + 1)
	}
	if err := query.Find(&nodes).Error; err != nil {
		return nil, "", err
	}

	var nextCursor string
	if filter.Limit > 0 && len(nodes) > filter.Limit {
		last := nodes[filter.Limit-1]
		nextCursor = encodeNodeCursor(last)
		nodes = nodes[:filter.Limit]
	}
	return nodes, nextCursor, nil
}

func (r *NodeRepository) FindIDsByFilter(filter model.NodeFilter) ([]uint, error) {
	var ids []uint
	return ids, r.buildFilteredQuery(filter).Pluck("id", &ids).Error
}

func (r *NodeRepository) FindByIDs(ids []uint) ([]*model.Node, error) {
	var nodes []*model.Node
	return nodes, r.db.Where("id IN ?", ids).Find(&nodes).Error
}

func (r *NodeRepository) FindTopNodes(limit int) ([]*model.Node, error) {
	var nodes []*model.Node
	return nodes, r.db.Where("latency >= ?", constants.LatencyMinValid).Order("latency ASC").Limit(limit).Find(&nodes).Error
}

func (r *NodeRepository) FindExistingIdentityKeys(nodes []*model.Node) (map[string]struct{}, error) {
	keys := make(map[string]struct{})
	endpoints := collectNodeEndpoints(nodes)
	if len(endpoints) == 0 {
		return keys, nil
	}

	for start := 0; start < len(endpoints); start += constants.NodeBatchSize {
		end := start + constants.NodeBatchSize
		if end > len(endpoints) {
			end = len(endpoints)
		}
		batch := endpoints[start:end]

		placeholders := make([]string, len(batch))
		args := make([]any, 0, len(batch)*2)
		for i, ep := range batch {
			placeholders[i] = "(?,?)"
			args = append(args, ep.Address, ep.Port)
		}
		query := r.db.Model(&model.Node{}).
			Select("address, port, protocol, raw_config, outbound_config").
			Where("(address, port) IN ("+strings.Join(placeholders, ",")+")", args...)

		var existing []model.Node
		if err := query.Find(&existing).Error; err != nil {
			return nil, err
		}
		for _, node := range existing {
			keys[node.IdentityKey()] = struct{}{}
		}
	}

	return keys, nil
}

func (r *NodeRepository) CountBySubscription(subscriptionID uint) (int64, error) {
	var count int64
	return count, r.db.Model(&model.Node{}).Where("subscription_id = ?", subscriptionID).Count(&count).Error
}

type NodeSummaryCounts struct {
	All       int64 `gorm:"column:total_count"`
	Available int64
	Pending   int64
	Timeout   int64
}

func (r *NodeRepository) CountByLatencyStatus() (*NodeSummaryCounts, error) {
	var counts NodeSummaryCounts
	err := r.db.Model(&model.Node{}).Select(`
		COUNT(*) AS total_count,
		SUM(CASE WHEN latency >= ? THEN 1 ELSE 0 END) AS available,
		SUM(CASE WHEN latency = ? OR latency IS NULL THEN 1 ELSE 0 END) AS pending,
		SUM(CASE WHEN latency = ? THEN 1 ELSE 0 END) AS timeout
	`, constants.LatencyMinValid, constants.LatencyUntested, constants.LatencyTimeout).Scan(&counts).Error
	return &counts, err
}

func (r *NodeRepository) SaveBatch(nodes []*model.Node) error {
	for i := 0; i < len(nodes); i += constants.NodeBatchSize {
		end := i + constants.NodeBatchSize
		if end > len(nodes) {
			end = len(nodes)
		}
		for _, node := range nodes[i:end] {
			node.LatencyRank = latencyRank(node.Latency)
		}
		if err := r.db.Create(nodes[i:end]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *NodeRepository) UpdateLatency(id uint, latency int64) error {
	return r.db.Model(&model.Node{}).Where("id = ?", id).Updates(map[string]any{"latency": latency, "latency_rank": latencyRank(latency)}).Error
}

func (r *NodeRepository) Delete(id uint) error { return r.db.Delete(&model.Node{}, id).Error }
func (r *NodeRepository) DeleteByLatency(latency int64) (int64, error) {
	result := r.db.Where("latency = ?", latency).Delete(&model.Node{})
	return result.RowsAffected, result.Error
}
func (r *NodeRepository) DeleteByFilterAndLatency(filter model.NodeFilter, latency int64) (int64, error) {
	query := r.db.Model(&model.Node{}).Where("latency = ?", latency)
	query = r.applySubscriptionFilter(query, filter)
	result := query.Delete(&model.Node{})
	return result.RowsAffected, result.Error
}

func (r *NodeRepository) buildFilteredQuery(filter model.NodeFilter) *gorm.DB {
	query := r.db.Model(&model.Node{})
	query = r.applySubscriptionFilter(query, filter)
	return r.applyLatencyStatusesFilter(query, filter.LatencyStatuses)
}

func (r *NodeRepository) applySubscriptionFilter(query *gorm.DB, filter model.NodeFilter) *gorm.DB {
	if filter.ExcludeID > 0 {
		query = query.Where("id <> ?", filter.ExcludeID)
	}
	if filter.SubscriptionID > 0 {
		query = query.Where("subscription_id = ?", filter.SubscriptionID)
	}
	if filter.Protocol != "" {
		query = query.Where("protocol = ?", filter.Protocol)
	}
	if filter.Keyword != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		portValue, portErr := strconv.Atoi(strings.TrimSpace(filter.Keyword))
		conditions := []string{
			"name LIKE ?",
			"address LIKE ?",
			"protocol LIKE ?",
			"raw_url LIKE ?",
		}
		args := []any{keyword, keyword, keyword, keyword}
		if portErr == nil {
			conditions = append(conditions, "port = ?")
			args = append(args, portValue)
		} else {
			conditions = append(conditions, "CAST(port AS TEXT) LIKE ?")
			args = append(args, keyword)
		}
		query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	}
	return query
}

func (r *NodeRepository) applyLatencyStatusesFilter(query *gorm.DB, statuses []string) *gorm.DB {
	if len(statuses) == 0 {
		return query
	}
	var conditions []string
	var args []any
	for _, status := range statuses {
		switch status {
		case constants.LatencyStatusPending:
			conditions = append(conditions, "(latency = ? OR latency IS NULL)")
			args = append(args, constants.LatencyUntested)
		case constants.LatencyStatusAvailable:
			conditions = append(conditions, "latency >= ?")
			args = append(args, constants.LatencyMinValid)
		case constants.LatencyStatusTimeout:
			conditions = append(conditions, "latency = ?")
			args = append(args, constants.LatencyTimeout)
		}
	}
	if len(conditions) == 0 {
		return query
	}
	return query.Where("("+strings.Join(conditions, " OR ")+")", args...)
}

func (r *NodeRepository) applyNodeSort(query *gorm.DB) *gorm.DB {
	return query.Order("latency_rank ASC").Order("latency ASC").Order("id ASC")
}

func (r *NodeRepository) applyNodeCursor(query *gorm.DB, cursorValue string) *gorm.DB {
	cursor, ok := decodeNodeCursor(cursorValue)
	if !ok {
		return query
	}

	return query.Where("latency_rank > ? OR (latency_rank = ? AND latency > ?) OR (latency_rank = ? AND latency = ? AND id > ?)", cursor.LatencyRank, cursor.LatencyRank, cursor.Latency, cursor.LatencyRank, cursor.Latency, cursor.ID)
}

func encodeNodeCursor(node *model.Node) string {
	return encodeCursor(nodeCursor{
		ID:          node.ID,
		Latency:     node.Latency,
		LatencyRank: node.LatencyRank,
	})
}

func decodeNodeCursor(value string) (nodeCursor, bool) {
	var cursor nodeCursor
	return cursor, decodeCursor(value, &cursor)
}

func latencyRank(latency int64) int {
	if latency >= constants.LatencyMinValid {
		return 0
	}
	if latency == constants.LatencyUntested {
		return 1
	}
	return 2
}

type nodeEndpoint struct {
	Address string
	Port    int
}

func collectNodeEndpoints(nodes []*model.Node) []nodeEndpoint {
	seen := make(map[string]struct{})
	endpoints := make([]nodeEndpoint, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		key := fmt.Sprintf("%s:%d", node.Address, node.Port)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		endpoints = append(endpoints, nodeEndpoint{Address: node.Address, Port: node.Port})
	}
	return endpoints
}

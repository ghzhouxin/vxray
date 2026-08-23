package repository

import (
	"fmt"
	"strconv"
	"strings"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/pkg/utils"

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
		cursor, err := encodeNodeCursor(last)
		if err != nil {
			return nil, "", err
		}
		nextCursor = cursor
		nodes = nodes[:filter.Limit]
	}
	return nodes, nextCursor, nil
}

// FindByFilterAll 按筛选条件返回全部节点（无分页、无置顶），测速用。
func (r *NodeRepository) FindByFilterAll(filter model.NodeFilter) ([]*model.Node, error) {
	var nodes []*model.Node
	return nodes, r.buildFilteredQuery(filter).Find(&nodes).Error
}

func (r *NodeRepository) FindByIDs(ids []uint) ([]*model.Node, error) {
	var nodes []*model.Node
	return nodes, r.db.Where("id IN ?", ids).Find(&nodes).Error
}

func (r *NodeRepository) FindTopNodes(limit int) ([]*model.Node, error) {
	var nodes []*model.Node
	query := r.db.Where("latency >= ?", constants.LatencyMinValid).Order("latency ASC").Limit(limit)
	return nodes, query.Find(&nodes).Error
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
			Select("address, port, protocol, raw_config, transport").
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

type LatencyUpdate struct {
	ID      uint
	Latency int64
}

// BatchUpdateLatency 用单条 CASE WHEN 批量更新，避免 N 条单行 UPDATE。
func (r *NodeRepository) BatchUpdateLatency(updates []LatencyUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	latencyCases := strings.Builder{}
	rankCases := strings.Builder{}
	ids := make([]uint, 0, len(updates))
	for _, u := range updates {
		latencyCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", u.ID, u.Latency))
		rankCases.WriteString(fmt.Sprintf("WHEN %d THEN %d ", u.ID, latencyRank(u.Latency)))
		ids = append(ids, u.ID)
	}

	sql := fmt.Sprintf(
		"UPDATE nodes SET latency = CASE id %sEND, latency_rank = CASE id %sEND WHERE id IN ?",
		latencyCases.String(), rankCases.String(),
	)
	return r.db.Exec(sql, ids).Error
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

func encodeNodeCursor(node *model.Node) (string, error) {
	return utils.EncodeCursor(nodeCursor{
		ID:          node.ID,
		Latency:     node.Latency,
		LatencyRank: node.LatencyRank,
	})
}

func decodeNodeCursor(value string) (nodeCursor, bool) {
	var cursor nodeCursor
	return cursor, utils.DecodeCursor(value, &cursor)
}

func latencyRank(latency int64) int {
	switch constants.LatencyStatus(latency) {
	case constants.LatencyStatusAvailable:
		return 0
	case constants.LatencyStatusTimeout:
		return 2
	default:
		return 1
	}
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
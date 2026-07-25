package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/dto"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/httpclient"
	"v2ray-server/pkg/subscription"
	"v2ray-server/pkg/types"

	"gorm.io/gorm"
)

type SubscriptionService struct {
	repo     *repository.SubscriptionRepository
	nodeRepo *repository.NodeRepository
	db       *gorm.DB
	parser   *subscription.Service
	logSvc   *LogService
	logger   *TaggedLogger
	client   *http.Client
}

type BatchUpdateResult = dto.BatchUpdateResult
type ParseStats = dto.ParseStats

func NewSubscriptionService(db *gorm.DB, logger *LogService, nodeRepo *repository.NodeRepository) *SubscriptionService {
	return &SubscriptionService{
		repo:     repository.NewSubscriptionRepository(db),
		nodeRepo: nodeRepo,
		db:       db,
		parser:   subscription.NewService(),
		logSvc:   logger,
		logger:   logger.NewTaggedLogger(constants.TagSubscription),
		client:   httpclient.LongRunning(),
	}
}

func (s *SubscriptionService) log(op string, id uint, name string, err error) {
	if err != nil {
		s.logger.Error(op+"失败", map[string]any{"id": id, "name": name, "error": err.Error()})
	} else {
		s.logger.Info(op+"成功", map[string]any{"id": id, "name": name})
	}
}

func (s *SubscriptionService) failUpdate(id uint, op *OperationLog, message string, err error) error {
	s.updateSyncStatus(id, "failed", nil)
	if op != nil {
		_ = op.Fail(message, map[string]any{"id": id, "error": err.Error()})
	}
	return err
}

func (s *SubscriptionService) List() ([]model.Subscription, error) {
	return s.repo.FindAllWithNodeCount()
}

func (s *SubscriptionService) Get(id uint) (*model.Subscription, error) {
	return s.repo.FindByIDWithNodeCount(id)
}

func (s *SubscriptionService) Create(sub *model.Subscription) error {
	err := s.repo.Create(sub)
	s.log("创建订阅", sub.ID, sub.Name, err)
	return err
}

func (s *SubscriptionService) Update(sub *model.Subscription) (*model.Subscription, error) {
	err := s.repo.Update(sub)
	s.log("更新订阅", sub.ID, sub.Name, err)
	if err != nil {
		return nil, err
	}
	return s.repo.FindByIDWithNodeCount(sub.ID)
}

func (s *SubscriptionService) Delete(id uint) error {
	sub, err := s.Get(id)
	if err != nil {
		return err
	}
	err = s.repo.DeleteWithNodes(id)
	s.log("删除订阅", id, sub.Name, err)
	return err
}

func (s *SubscriptionService) UpdateNodesBatch(ctx context.Context, ids []uint) (*BatchUpdateResult, error) {
	targetIDs := ids
	if len(targetIDs) == 0 {
		subs, err := s.List()
		if err != nil {
			return nil, err
		}
		targetIDs = make([]uint, 0, len(subs))
		for _, sub := range subs {
			targetIDs = append(targetIDs, sub.ID)
		}
	}

	result := &BatchUpdateResult{Total: len(targetIDs)}
	for _, id := range targetIDs {
		if _, err := s.UpdateNodes(ctx, id); err != nil {
			result.Failed++
			continue
		}
		result.Success++
	}
	return result, nil
}

func (s *SubscriptionService) UpdateNodes(ctx context.Context, id uint) (*ParseStats, error) {
	sub, err := s.Get(id)
	if err != nil {
		s.logger.Error("获取订阅失败", map[string]any{"id": id, "error": err.Error()})
		return nil, err
	}
	var op *OperationLog
	if s.logSvc != nil {
		var opErr error
		op, opErr = s.logSvc.StartOperation(constants.TagSubscription, "开始更新订阅", map[string]any{"id": id, "name": sub.Name})
		if opErr != nil {
			log.Printf("SubscriptionService: 启动操作日志失败 id=%d name=%s err=%v", id, sub.Name, opErr)
		}
	}

	body, err := s.fetchContent(ctx, sub.URL, id)
	if err != nil {
		return nil, s.failUpdate(id, op, "订阅更新失败", err)
	}
	if op != nil {
		_ = op.Update("订阅内容拉取成功", map[string]any{"id": id, "name": sub.Name, "bytes": len(body)})
	}

	contentHash := s.calculateHash(body)
	unchanged, err := s.contentUnchanged(sub, contentHash, id)
	if err != nil {
		return nil, s.failUpdate(id, op, "检查订阅内容变更失败", err)
	}
	if unchanged {
		now := time.Now()
		s.updateSyncStatus(id, "success", &now)
		if op != nil {
			_ = op.Success("订阅更新完成", map[string]any{"id": id, "name": sub.Name, "unchanged": true})
		}
		return &ParseStats{Unchanged: true}, nil
	}

	result, err := s.parseWithStats(string(body))
	if err != nil {
		return nil, s.failUpdate(id, op, "解析订阅失败", err)
	}
	if op != nil {
		_ = op.Update("订阅解析完成", map[string]any{"id": id, "name": sub.Name, "total": result.Total})
	}

	newNodes, err := s.filterNewNodes(result.Nodes, id)
	if err != nil {
		return nil, s.failUpdate(id, op, "读取现有节点失败", err)
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(newNodes) > 0 {
			if err := repository.NewNodeRepository(tx).SaveBatch(newNodes); err != nil {
				return err
			}
		}
		return repository.NewSubscriptionRepository(tx).UpdateContentHash(id, contentHash)
	}); err != nil {
		return nil, s.failUpdate(id, op, "写入节点失败", err)
	}

	stats := &ParseStats{Total: result.Total, Success: result.Total, Duplicates: result.Total - len(newNodes), Added: len(newNodes)}
	now := time.Now()
	s.updateSyncStatus(id, "success", &now)
	if op != nil {
		_ = op.Success("订阅更新完成", map[string]any{
			"id":         id,
			"name":       sub.Name,
			"total":      stats.Total,
			"duplicates": stats.Duplicates,
			"added":      stats.Added,
		})
	}
	return stats, nil
}

func (s *SubscriptionService) fetchContent(ctx context.Context, url string, id uint) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch subscription failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: read body: %w", err)
	}
	return body, nil
}

func (s *SubscriptionService) calculateHash(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

func (s *SubscriptionService) contentUnchanged(sub *model.Subscription, contentHash string, id uint) (bool, error) {
	if sub.ContentHash != "" && sub.ContentHash == contentHash {
		count, err := s.nodeRepo.CountBySubscription(id)
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}
	return false, nil
}

func (s *SubscriptionService) updateSyncStatus(id uint, status string, syncedAt *time.Time) {
	if err := s.repo.UpdateSyncStatus(id, status, syncedAt); err != nil {
		s.logger.Error("更新订阅同步状态失败", map[string]any{"id": id, "status": status, "error": err.Error()})
	}
}

func (s *SubscriptionService) filterNewNodes(nodes []*model.Node, subscriptionID uint) ([]*model.Node, error) {
	existingKeys, err := s.nodeRepo.FindExistingIdentityKeys(nodes)
	if err != nil {
		s.logger.Error("读取现有节点失败", map[string]any{"subscription_id": subscriptionID, "error": err.Error()})
		return nil, err
	}
	var newNodes []*model.Node
	batchKeys := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		node.SubscriptionID = subscriptionID
		key := node.IdentityKey()
		if _, duplicated := batchKeys[key]; duplicated {
			continue
		}
		batchKeys[key] = struct{}{}
		if _, exists := existingKeys[key]; exists {
			continue
		}
		newNodes = append(newNodes, node)
	}
	return newNodes, nil
}

type parseResult struct {
	Nodes []*model.Node
	Total int
}

func (s *SubscriptionService) parseWithStats(content string) (*parseResult, error) {
	urls := subscription.CleanContent(content)
	parsedResult := s.parser.ParseNodesWithDedup(urls)
	nodes := make([]*model.Node, 0, len(parsedResult.Nodes))
	for _, n := range parsedResult.Nodes {
		nodes = append(nodes, convertParsedNodeToModel(n))
	}
	return &parseResult{Nodes: nodes, Total: parsedResult.Total}, nil
}

func convertParsedNodeToModel(n *types.ParsedNode) *model.Node {
	if n == nil {
		return nil
	}
	name := n.Name
	if name == "" {
		name = n.Address
	}
	return &model.Node{Name: name, Protocol: n.Protocol, Address: n.Address, Port: n.Port, RawURL: n.RawURL, RawConfig: n.RawConfig, OutboundConfig: n.OutboundConfig}
}

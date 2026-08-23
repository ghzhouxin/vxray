package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"v2ray-server/internal/constants"
	"v2ray-server/internal/model"
	"v2ray-server/internal/repository"
	"v2ray-server/pkg/subscription"
	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"

	"gorm.io/gorm"
)

type SubscriptionService struct {
	repo     *repository.SubscriptionRepository
	nodeRepo *repository.NodeRepository
	logger   *TaggedLogger
	client   *http.Client
	batchBus *ProgressBus
}

// BatchUpdateResult 汇总批量订阅更新的结果。
type BatchUpdateResult struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
}

func NewSubscriptionService(db *gorm.DB, logSvc *LogService, nodeRepo *repository.NodeRepository) *SubscriptionService {
	return &SubscriptionService{
		repo:     repository.NewSubscriptionRepository(db),
		nodeRepo: nodeRepo,
		logger:   logSvc.NewTaggedLogger(constants.TagSubscription),
		client:   utils.LongRunningHTTPClient(),
	}
}

// PrepareBatchBus 创建新的进度总线并设置到服务上，供 SSE handler 在启动批量更新前订阅。
func (s *SubscriptionService) PrepareBatchBus() *ProgressBus {
	bus := NewProgressBus()
	bus.Start()
	s.batchBus = bus
	return bus
}

func (s *SubscriptionService) log(op string, id uint, name string, err error) {
	if err != nil {
		s.logger.Error(op+"失败", map[string]any{"id": id, "name": name, "error": err.Error()})
	} else {
		s.logger.Info(op+"成功", map[string]any{"id": id, "name": name})
	}
}

func (s *SubscriptionService) fail(id uint, stage string, err error, start time.Time) error {
	s.updateSyncStatus(id, "failed", nil)
	s.logger.Error(stage+"失败", map[string]any{"id": id, "error": err.Error(), "total_ms": time.Since(start).Milliseconds()})
	return err
}

func (s *SubscriptionService) markSuccess(id uint, name, msg string, start time.Time) {
	now := time.Now()
	s.updateSyncStatus(id, "success", &now)
	s.logger.Info("订阅更新"+msg, map[string]any{"id": id, "name": name, "total_ms": time.Since(start).Milliseconds()})
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
	bus := s.batchBus
	if bus == nil {
		bus = NewProgressBus()
		bus.Start()
		s.batchBus = bus
	}

	prog := func(completed int, id uint, msg, errMsg string) {
		bus.Publish(OperationProgress{
			Type:      "subscription_update",
			Status:    "running",
			Total:     result.Total,
			Completed: completed,
			Success:   result.Success,
			Failed:    result.Failed,
			NodeID:    id,
			Message:   msg,
			Error:     errMsg,
		}, false)
	}

	prog(0, 0, "开始更新订阅", "")
	for i, id := range targetIDs {
		prog(i, id, "正在更新订阅", "")
		if err := s.UpdateNodes(ctx, id); err != nil {
			result.Failed++
			prog(i+1, id, "订阅更新失败", err.Error())
			continue
		}
		result.Success++
		prog(i+1, id, "订阅更新成功", "")
	}

	finalStatus := "success"
	if result.Failed > 0 && result.Success == 0 {
		finalStatus = "failed"
	}
	bus.Publish(OperationProgress{
		Type:      "subscription_update",
		Status:    finalStatus,
		Total:     result.Total,
		Completed: result.Total,
		Success:   result.Success,
		Failed:    result.Failed,
		Message:   "订阅批量更新完成",
	}, true)
	return result, nil
}

func (s *SubscriptionService) UpdateNodes(ctx context.Context, id uint) error {
	start := time.Now()

	sub, err := s.Get(id)
	if err != nil {
		return s.fail(id, "获取订阅", err, start)
	}

	body, err := s.fetchContent(ctx, sub.URL)
	if err != nil {
		return s.fail(id, "拉取订阅", err, start)
	}

	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	if sub.ContentHash == hash {
		if count, _ := s.nodeRepo.CountBySubscription(id); count > 0 {
			s.markSuccess(id, sub.Name, "内容未变更", start)
			return nil
		}
	}

	urls := subscription.CleanContent(string(body))
	parsed, failed := subscription.ParseNodesWithDedup(urls)
	if len(parsed) == 0 && len(urls) > 0 {
		return s.fail(id, "解析订阅", fmt.Errorf("%d 个 URL 全部解析失败", len(urls)), start)
	}

	nodes := make([]*model.Node, 0, len(parsed))
	for _, n := range parsed {
		nodes = append(nodes, convertParsedNodeToModel(n))
	}
	newNodes, err := s.filterNewNodes(nodes, id)
	if err != nil {
		return s.fail(id, "读取现有节点", err, start)
	}

	if err := s.repo.SaveNodesAndContentHash(newNodes, id, hash); err != nil {
		return s.fail(id, "写入节点", err, start)
	}

	msg := fmt.Sprintf("完成: %d 解析, %d 新增", len(parsed), len(newNodes))
	if failed > 0 {
		msg += fmt.Sprintf(", %d 解析失败", failed)
	}
	s.markSuccess(id, sub.Name, msg, start)
	return nil
}

func (s *SubscriptionService) fetchContent(ctx context.Context, url string) ([]byte, error) {
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

func (s *SubscriptionService) updateSyncStatus(id uint, status string, syncedAt *time.Time) {
	if err := s.repo.UpdateSyncStatus(id, status, syncedAt); err != nil {
		s.logger.Error("更新订阅同步状态失败", map[string]any{"id": id, "status": status, "error": err.Error()})
	}
}

func (s *SubscriptionService) filterNewNodes(nodes []*model.Node, subscriptionID uint) ([]*model.Node, error) {
	existingKeys, err := s.nodeRepo.FindExistingIdentityKeys(nodes)
	if err != nil {
		return nil, err
	}
	var newNodes []*model.Node
	batchKeys := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
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

func convertParsedNodeToModel(n *types.ParsedNode) *model.Node {
	return &model.Node{Name: n.Name, Protocol: n.Protocol, Address: n.Address, Port: n.Port, RawURL: n.RawURL, RawConfig: n.RawConfig, Transport: n.Transport}
}

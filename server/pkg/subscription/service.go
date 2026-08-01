package subscription

import (
	"log"
	"runtime/debug"
	"strings"
	"sync"

	"v2ray-server/pkg/types"
)

const maxParseConcurrency = 32

// protocolPrefix 提取协议前缀用于日志，避免暴露 trojan://password@host 等凭证
func protocolPrefix(url string) string {
	for _, p := range types.ProtocolPrefixes {
		if strings.HasPrefix(url, p) {
			return p
		}
	}
	return "unknown://"
}

// ParseNodesWithDedup 并发解析节点 URL，按 IdentityKey 去重。
func ParseNodesWithDedup(urls []string) *types.ParseResult {
	type slot struct {
		node *types.ParsedNode
		ok   bool
	}
	results := make([]slot, len(urls))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxParseConcurrency) // limit concurrency to avoid exhausting resources

	for i, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, url string) {
			defer func() { <-sem; wg.Done() }()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("subscription: parse panic for %s: %v\n%s", protocolPrefix(url), r, debug.Stack())
				}
			}()

			node, err := Parse(url)
			if err != nil {
				log.Printf("subscription: parse failed for %s: %v", protocolPrefix(url), err)
				return
			}
			results[idx] = slot{node: node, ok: true}
		}(i, u)
	}

	wg.Wait()

	nodesMap := make(map[string]*types.ParsedNode)
	nodes := make([]*types.ParsedNode, 0, len(urls))
	for _, s := range results {
		if !s.ok {
			continue
		}
		key := s.node.IdentityKey()
		if _, exists := nodesMap[key]; !exists {
			nodesMap[key] = s.node
			nodes = append(nodes, s.node)
		}
	}

	return &types.ParseResult{
		Nodes: nodes,
		Total: len(urls),
	}
}

package subscription

import (
	"fmt"
	"html"
	"strconv"
	"strings"

	"v2ray-server/pkg/types"
	"v2ray-server/pkg/utils"
)

func Parse(nodeURL string) (*types.ParsedNode, error) {
	nodeURL = utils.CleanNodeURL(nodeURL)
	nodeURL = strings.ReplaceAll(nodeURL, "&amp%3B", "&")
	nodeURL = html.UnescapeString(nodeURL)
	nodeURL = strings.ReplaceAll(nodeURL, "&;", "&") // 修复订阅源 ?security=reality&;encryption=none 格式
	nodeURL = fixStrayPercent(nodeURL)
	nodeURL = encodeHashInQueryValues(nodeURL)
	nodeURL = fixIllegalUserinfoChars(nodeURL)

	var node *types.ParsedNode
	var err error
	switch {
	case strings.HasPrefix(nodeURL, types.PrefixVMess):
		node, err = parseVMess(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixVLESS):
		node, err = parseVLESS(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixTrojan):
		node, err = parseTrojan(nodeURL)
	case strings.HasPrefix(nodeURL, types.PrefixSS):
		node, err = parseSS(nodeURL)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", nodeURL)
	}
	if err != nil {
		return nil, err
	}
	node.RawURL = nodeURL
	return node, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizePort(port int) int {
	if port == 0 {
		return DefaultPort
	}
	return port
}

func portFromAny(v any) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		result, _ := strconv.Atoi(val)
		return result
	default:
		return 0
	}
}

func newParsedNode(name, protocol, address string, port int, rawConfig types.Map, transport types.Transport) *types.ParsedNode {
	if name == "" {
		name = address
	}
	return &types.ParsedNode{
		Name:      name,
		Protocol:  protocol,
		Address:   address,
		Port:      port,
		RawConfig: rawConfig,
		Transport: transport,
	}
}

func normalizeNetwork(network string) string {
	if network == "" || network == types.NetworkRaw {
		return types.NetworkTCP // 空值或 xray-core v25+ raw 别名归一化为 tcp
	}
	return network
}

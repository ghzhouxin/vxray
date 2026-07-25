package subscription

import (
	"net/url"
	"strings"

	"v2ray-server/pkg/types"
)

func buildTLSSettings(sni, fp string, alpn any) types.Map {
	tlsSettings := types.Map{}
	if sni != "" {
		tlsSettings["serverName"] = sni
	}
	if fp != "" {
		tlsSettings["fingerprint"] = fp
	}
	if alpnList := normalizeALPN(alpn); len(alpnList) > 0 {
		tlsSettings["alpn"] = alpnList
	}
	return tlsSettings
}

// normalizeALPN 处理 query（string）或 VMess JSON（string/[]any/[]string）的 alpn
func normalizeALPN(v any) []string {
	var raw []string
	switch a := v.(type) {
	case string:
		if a == "" {
			return nil
		}
		for _, p := range strings.Split(a, ",") {
			raw = append(raw, strings.TrimSpace(p))
		}
	case []any:
		for _, item := range a {
			if s, ok := item.(string); ok {
				raw = append(raw, s)
			}
		}
	case []string:
		raw = a
	default:
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// resolveSNI: sni → peer → host（VMess JSON 用 resolveVMessSNI，无 peer）
func resolveSNI(query url.Values) string {
	return firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("host"))
}

// buildTLSSettingsFromQuery 不解析 allowInsecure：xray-core v26 会 PrintRemovedFeatureError。
// 用 vcn/pcs 替代证书验证。
func buildTLSSettingsFromQuery(query url.Values) types.Map {
	tlsSettings := buildTLSSettings(resolveSNI(query), query.Get("fp"), query.Get("alpn"))
	if ech := query.Get("ech"); ech != "" {
		tlsSettings["echConfigList"] = ech
	}
	applyTLSCertVerification(tlsSettings, query.Get("vcn"), query.Get("pcs"))
	return tlsSettings
}

// applyTLSCertVerification 设置证书固定字段（vcn/pcs）
func applyTLSCertVerification(tlsSettings types.Map, vcn, pcs string) {
	if vcn != "" {
		tlsSettings["verifyPeerCertByName"] = vcn
	}
	if pcs != "" {
		tlsSettings["pinnedPeerCertSha256"] = pcs
	}
}

func buildRealitySettings(query url.Values) types.Map {
	realitySettings := types.Map{}
	if sni := resolveSNI(query); sni != "" {
		realitySettings["serverName"] = sni
	}
	if v := firstNonEmpty(query.Get("publicKey"), query.Get("pbk")); v != "" {
		realitySettings["publicKey"] = v
	}
	if v := firstNonEmpty(query.Get("shortId"), query.Get("sid")); v != "" {
		realitySettings["shortId"] = v
	}
	if v := firstNonEmpty(query.Get("fingerprint"), query.Get("fp")); v != "" {
		realitySettings["fingerprint"] = v
	} else {
		realitySettings["fingerprint"] = DefaultRealityFingerprint
	}
	if v := firstNonEmpty(query.Get("spiderX"), query.Get("spx")); v != "" {
		realitySettings["spiderX"] = v
	}
	// mldsa65Verify: REALITY 后量子验证（xray-core REALITYConfig.Mldsa65Verify）
	if v := firstNonEmpty(query.Get("mldsa65Verify"), query.Get("pqv")); v != "" {
		realitySettings["mldsa65Verify"] = v
	}
	return realitySettings
}

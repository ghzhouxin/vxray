package subscription

import (
	"net/url"
	"strings"

	"v2ray-server/pkg/types"
)

func buildTLS(query url.Values) *types.TLSConfig {
	sni := resolveSNI(query)
	ech := query.Get("ech")
	vcn := query.Get("vcn")
	pcs := query.Get("pcs")
	if sni == "" && query.Get("fp") == "" && query.Get("alpn") == "" && ech == "" && vcn == "" && pcs == "" {
		return nil
	}
	cfg := &types.TLSConfig{}
	if sni != "" {
		cfg.ServerName = sni
	}
	if fp := query.Get("fp"); fp != "" {
		cfg.Fingerprint = fp
	}
	if alpnList := normalizeALPN(query.Get("alpn")); len(alpnList) > 0 {
		cfg.ALPN = alpnList
	}
	if ech != "" {
		cfg.ECHConfigList = ech
	}
	if vcn != "" {
		cfg.VerifyPeerCertByName = vcn
	}
	if pcs != "" {
		cfg.PinnedPeerCertSha256 = pcs
	}
	return cfg
}

func buildReality(query url.Values) *types.RealityConfig {
	cfg := &types.RealityConfig{}
	hasAny := false
	if sni := resolveSNI(query); sni != "" {
		cfg.ServerName = sni
		hasAny = true
	}
	if v := firstNonEmpty(query.Get("publicKey"), query.Get("pbk")); v != "" {
		cfg.PublicKey = v
		hasAny = true
	}
	if v := firstNonEmpty(query.Get("shortId"), query.Get("sid")); v != "" {
		cfg.ShortID = v
		hasAny = true
	}
	if v := query.Get("spx"); v != "" {
		cfg.SpiderX = v
		hasAny = true
	}
	if v := query.Get("pqv"); v != "" {
		cfg.Mldsa65Verify = v
		hasAny = true
	}
	if v := firstNonEmpty(query.Get("fingerprint"), query.Get("fp")); v != "" {
		cfg.Fingerprint = v
		hasAny = true
	} else {
		cfg.Fingerprint = DefaultRealityFingerprint
	}
	if !hasAny {
		return nil
	}
	return cfg
}

// resolveSNI: sni → peer → host
func resolveSNI(query url.Values) string {
	return firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("host"))
}

// normalizeALPN 处理 query（string）或 VMess JSON（string/[]any）的 alpn
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

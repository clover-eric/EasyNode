package recommender

import "easynode/internal/model"

func Recommend(env model.Environment) []model.Recommendation {
	recs := []model.Recommendation{
		{Protocol: "vless-reality", Transport: "tcp", Security: "reality", Priority: 5, Label: "抗封锁首选", Description: "Reality，可立即使用，默认开启", Enabled: env.HasIPv4},
		{Protocol: "shadowsocks", Transport: "tcp-udp", Security: "aead", Priority: 5, Label: "Clash 兼容", Description: "Clash 原生支持，IP 直连模式默认可用", Enabled: env.HasIPv4},
		{Protocol: "hysteria2", Transport: "quic", Security: "tls", Priority: 5, Label: "高速传输", Description: "UDP 高速传输，证书就绪后自动启用", Enabled: env.TLSReady && env.UDPAvailable},
		{Protocol: "trojan-tls", Transport: "tcp", Security: "tls", Priority: 4, Label: "广泛兼容", Description: "TLS 标准形态，证书就绪后自动启用", Enabled: env.TLSReady},
		{Protocol: "clash", Transport: "rule", Security: "client-profile", Priority: 4, Label: "Clash 分流", Description: "Clash/Mihomo 订阅，自动测速和精准分流", Enabled: false},
		{Protocol: "vless-ws-tls", Transport: "ws", Security: "tls", Priority: 3, Label: "CDN 备用", Description: "适合接入 CDN 或反向代理", Enabled: false},
		{Protocol: "tuic", Transport: "quic", Security: "tls", Priority: 3, Label: "QUIC 备用", Description: "适合 UDP 通畅的网络环境", Enabled: false},
	}
	return recs
}

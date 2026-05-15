package recommender

import "easynode/internal/model"

func Recommend(env model.Environment) []model.Recommendation {
	recs := []model.Recommendation{
		{Protocol: "vless-reality", Transport: "tcp", Security: "reality", Priority: 5, Label: "抗封锁首选", Description: "Reality + Vision，可立即使用，默认开启", Enabled: env.HasIPv4},
		{Protocol: "hysteria2", Transport: "quic", Security: "tls", Priority: 5, Label: "高速传输", Description: "UDP 高速传输，证书就绪后自动启用", Enabled: env.TLSReady && env.UDPAvailable},
		{Protocol: "trojan-tls", Transport: "tcp", Security: "tls", Priority: 4, Label: "广泛兼容", Description: "TLS 标准形态，证书就绪后自动启用", Enabled: env.TLSReady},
		{Protocol: "vless-ws-tls", Transport: "ws", Security: "tls", Priority: 3, Label: "CDN 备用", Description: "适合接入 CDN 或反向代理", Enabled: false},
		{Protocol: "tuic", Transport: "quic", Security: "tls", Priority: 3, Label: "QUIC 备选", Description: "适合 UDP 通畅的网络环境", Enabled: false},
	}
	return recs
}

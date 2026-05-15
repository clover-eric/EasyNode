package model

import "time"

type AppState struct {
	SetupDone         bool          `json:"setup_done"`
	PanelPath         string        `json:"panel_path"`
	AdminPassword     string        `json:"-"`
	AdminPasswordHash string        `json:"admin_password_hash,omitempty"`
	SessionToken      string        `json:"session_token,omitempty"`
	LoginFailures     int           `json:"login_failures,omitempty"`
	LockoutUntil      time.Time     `json:"lockout_until,omitempty"`
	Domain            string        `json:"domain"`
	IPDirect          bool          `json:"ip_direct"`
	SubscribeKey      string        `json:"subscribe_key"`
	Nodes             []Node        `json:"nodes"`
	ChainPeers        []ChainPeer   `json:"chain_peers"`
	PairingCodes      []PairingCode `json:"-"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type Node struct {
	ID                string    `json:"id"`
	Protocol          string    `json:"protocol"`
	Transport         string    `json:"transport"`
	Security          string    `json:"security"`
	Label             string    `json:"label"`
	Description       string    `json:"description"`
	Priority          int       `json:"priority"`
	Status            string    `json:"status"`
	LatencyMS         *int      `json:"latency_ms"`
	TrafficUsed       int64     `json:"traffic_used"`
	TrafficTotal      *int64    `json:"traffic_total"`
	Port              int       `json:"port"`
	UUID              string    `json:"uuid"`
	Password          string    `json:"password"`
	RealityPrivateKey string    `json:"reality_private_key,omitempty"`
	RealityPublicKey  string    `json:"reality_public_key,omitempty"`
	RealityShortID    string    `json:"reality_short_id,omitempty"`
	Host              string    `json:"host"`
	CreatedAt         time.Time `json:"created_at"`
	SubscribeLink     string    `json:"subscribe_link"`
}

type Recommendation struct {
	Protocol    string `json:"protocol"`
	Transport   string `json:"transport"`
	Security    string `json:"security"`
	Priority    int    `json:"priority"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type Environment struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	PublicIP     string `json:"public_ip"`
	Domain       string `json:"domain"`
	IPDirect     bool   `json:"ip_direct"`
	HasIPv4      bool   `json:"has_ipv4"`
	UDPAvailable bool   `json:"udp_available"`
	TLSReady     bool   `json:"tls_ready"`
}

type ChainPeer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"`
	PublicKey string    `json:"public_key"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type PairingCode struct {
	Code      string    `json:"code"`
	Endpoint  string    `json:"endpoint"`
	PublicKey string    `json:"public_key"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

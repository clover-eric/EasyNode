package model

import "time"

type AppState struct {
	SetupDone            bool          `json:"setup_done"`
	PanelPath            string        `json:"panel_path"`
	AdminPassword        string        `json:"-"`
	AdminPasswordHash    string        `json:"admin_password_hash,omitempty"`
	SessionToken         string        `json:"session_token,omitempty"`
	LoginFailures        int           `json:"login_failures,omitempty"`
	LockoutUntil         time.Time     `json:"lockout_until,omitempty"`
	Domain               string        `json:"domain"`
	IPDirect             bool          `json:"ip_direct"`
	CertReady            bool          `json:"cert_ready"`
	CertPath             string        `json:"cert_path,omitempty"`
	KeyPath              string        `json:"key_path,omitempty"`
	SubscribeKey         string        `json:"subscribe_key"`
	Nodes                []Node        `json:"nodes"`
	ChainPeers           []ChainPeer   `json:"chain_peers"`
	ChainClients         []ChainClient `json:"chain_clients,omitempty"`
	ChainPairingDisabled bool          `json:"chain_pairing_disabled,omitempty"`
	PairingCodes         []PairingCode `json:"pairing_codes,omitempty"`
	UpdatedAt            time.Time     `json:"updated_at"`
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
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Endpoint              string    `json:"endpoint"`
	PublicKey             string    `json:"public_key"`
	OutboundLink          string    `json:"outbound_link,omitempty"`
	Status                string    `json:"status"`
	RemoteLatencyMS       *int      `json:"remote_latency_ms,omitempty"`
	RemotePairingDisabled bool      `json:"remote_pairing_disabled,omitempty"`
	RemoteStatusCheckedAt time.Time `json:"remote_status_checked_at,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

type ChainClient struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Endpoint        string    `json:"endpoint"`
	PublicKey       string    `json:"public_key"`
	OutboundLink    string    `json:"outbound_link,omitempty"`
	Status          string    `json:"status"`
	RemoteLatencyMS *int      `json:"remote_latency_ms,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type PairingCode struct {
	Code         string    `json:"code"`
	Endpoint     string    `json:"endpoint"`
	PublicKey    string    `json:"public_key"`
	OutboundLink string    `json:"outbound_link,omitempty"`
	Bundle       string    `json:"bundle,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Used         bool      `json:"used"`
}

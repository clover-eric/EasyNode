package api

import (
	"net/url"
	"sync"
	"time"

	"easynode/internal/core/detector"
	"easynode/internal/core/traffic"
	"easynode/internal/model"
	"easynode/internal/store"
)

type metricsCache struct {
	mu              sync.RWMutex
	portBytes       map[int]int64
	mainlandLatency *int
	peerStatus      map[string]peerRemoteStatus
	lastRefresh     time.Time
}

type peerRemoteStatus struct {
	disabled  bool
	checkedAt time.Time
	latencyMS *int
}

func startMetricsLoop(st *store.Store) *metricsCache {
	mc := &metricsCache{
		portBytes:  make(map[int]int64),
		peerStatus: make(map[string]peerRemoteStatus),
	}
	mc.refresh(st)
	go mc.loop(st)
	go mc.trafficPersistLoop(st)
	return mc
}

func (mc *metricsCache) loop(st *store.Store) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		mc.refresh(st)
	}
}

func (mc *metricsCache) refresh(st *store.Store) {
	portBytes := traffic.PortBytes()
	mainlandLatency := detector.MainlandLatency()

	snap := st.Snapshot()
	peerStatus := make(map[string]peerRemoteStatus, len(snap.ChainPeers))
	for _, peer := range snap.ChainPeers {
		disabled, checked, latency := remotePairingStatus(peer.Endpoint)
		peerStatus[peer.ID] = peerRemoteStatus{
			disabled:  disabled,
			checkedAt: checked,
			latencyMS: latency,
		}
	}

	mc.mu.Lock()
	mc.portBytes = portBytes
	mc.mainlandLatency = mainlandLatency
	mc.peerStatus = peerStatus
	mc.lastRefresh = time.Now()
	mc.mu.Unlock()
}

func (mc *metricsCache) enrich(st model.AppState) model.AppState {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	for i := range st.Nodes {
		st.Nodes[i].TrafficUsed = mc.portBytes[st.Nodes[i].Port]
		if st.Nodes[i].Status == "running" {
			st.Nodes[i].LatencyMS = mc.mainlandLatency
		} else {
			st.Nodes[i].LatencyMS = nil
		}
	}
	for i := range st.ChainPeers {
		if ps, ok := mc.peerStatus[st.ChainPeers[i].ID]; ok {
			st.ChainPeers[i].RemotePairingDisabled = ps.disabled
			st.ChainPeers[i].RemoteStatusCheckedAt = ps.checkedAt
			st.ChainPeers[i].RemoteLatencyMS = ps.latencyMS
		}
	}
	for i := range st.ChainClients {
		st.ChainClients[i].RemoteLatencyMS = detector.EndpointLatency(st.ChainClients[i].Endpoint)
	}
	return st
}

func remotePairingStatus(endpoint string) (bool, time.Time, *int) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, time.Time{}, nil
	}
	u.Path = "/api/v1/chain/public/status"
	u.RawQuery = ""
	return fetchRemotePairingStatus(u.String())
}

const maxTrafficHistory = 288 // 24h at 5-min intervals

func (mc *metricsCache) trafficPersistLoop(st *store.Store) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	var prev map[int]int64
	for range ticker.C {
		mc.mu.RLock()
		current := make(map[int]int64, len(mc.portBytes))
		for k, v := range mc.portBytes {
			current[k] = v
		}
		mc.mu.RUnlock()

		if prev == nil {
			prev = current
			continue
		}

		now := time.Now()
		var snaps []model.TrafficSnap
		for port, bytes := range current {
			delta := bytes - prev[port]
			if delta > 0 {
				snaps = append(snaps, model.TrafficSnap{Port: port, Bytes: delta, Timestamp: now})
			}
		}
		prev = current

		if len(snaps) == 0 {
			continue
		}

		_ = st.Update(func(s *model.AppState) error {
			s.TrafficHistory = append(s.TrafficHistory, snaps...)
			if len(s.TrafficHistory) > maxTrafficHistory {
				s.TrafficHistory = s.TrafficHistory[len(s.TrafficHistory)-maxTrafficHistory:]
			}
			return nil
		})
	}
}

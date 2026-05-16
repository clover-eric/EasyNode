package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"easynode/internal/core/chain"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/model"
	"easynode/internal/util"
)

func (s *Server) ChainPublicStatus(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"pairing_disabled": st.ChainPairingDisabled,
		"panel_path":       st.PanelPath,
		"domain":           st.Domain,
		"updated_at":       st.UpdatedAt,
	})
}

func (s *Server) ChainPublicPaired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Code         string `json:"code"`
		Endpoint     string `json:"endpoint"`
		PublicKey    string `json:"public_key"`
		OutboundLink string `json:"outbound_link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		if st.ChainPairingDisabled {
			return errors.New("chain pairing is disabled on this server")
		}
		found := false
		for i := range st.PairingCodes {
			if st.PairingCodes[i].Code == req.Code && st.PairingCodes[i].ExpiresAt.After(time.Now()) {
				st.PairingCodes[i].Used = true
				found = true
				break
			}
		}
		if !found && req.OutboundLink != "" {
			for _, n := range st.Nodes {
				if n.Status == "running" && subscribe.Link(n) == req.OutboundLink {
					found = true
					break
				}
			}
		}
		if !found {
			return errors.New("pairing code invalid or expired")
		}
		upsertChainClient(st, model.ChainClient{ID: util.Token(6), Name: chainClientName(req.Endpoint), Endpoint: req.Endpoint, PublicKey: req.PublicKey, OutboundLink: req.OutboundLink, Status: "paired", CreatedAt: time.Now()})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}


func (s *Server) ChainPublicUnpaired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ExitEndpoint string `json:"exit_endpoint"`
		Endpoint     string `json:"endpoint"`
		PublicKey    string `json:"public_key"`
		OutboundLink string `json:"outbound_link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	removed := false
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainPeers[:0]
		for _, peer := range st.ChainPeers {
			if chainPeerMatches(peer, req.ExitEndpoint, req.PublicKey, req.OutboundLink) {
				removed = true
				continue
			}
			next = append(next, peer)
		}
		st.ChainPeers = next
		nextClients := st.ChainClients[:0]
		for _, client := range st.ChainClients {
			if chainClientMatches(client, req.Endpoint, req.PublicKey) {
				removed = true
				continue
			}
			nextClients = append(nextClients, client)
		}
		st.ChainClients = nextClients
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if removed {
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
		_ = singbox.RestartService()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "removed": removed})
}


func (s *Server) GenerateCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	st := s.store.Snapshot()
	if st.ChainPairingDisabled {
		writeError(w, http.StatusForbidden, errors.New("chain pairing is disabled on this server"))
		return
	}
	outboundLink := firstRunningLink(st.Nodes)
	if outboundLink == "" {
		writeError(w, http.StatusBadRequest, errors.New("no running node available for chain exit"))
		return
	}
	code := chain.NewCode(req.Endpoint, outboundLink)
	err := s.store.Update(func(st *model.AppState) error {
		st.PairingCodes = append(st.PairingCodes, code)
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, code)
}


func (s *Server) Pair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Code        string `json:"code"`
		Endpoint    string `json:"my_endpoint"`
		PublicKey   string `json:"my_public_key"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if bundle, err := chain.DecodeBundle(strings.TrimSpace(req.Code)); err == nil {
		name := chainPeerName(req.DisplayName, bundle.Endpoint)
		err := s.store.Update(func(st *model.AppState) error {
			upsertChainPeer(st, model.ChainPeer{ID: util.Token(6), Name: name, Endpoint: bundle.Endpoint, PublicKey: bundle.PublicKey, OutboundLink: bundle.OutboundLink, Status: "paired", CreatedAt: time.Now()})
			return nil
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
		_ = singbox.RestartService()
		notified, notifyErr := notifyExitPaired(bundle, req.Endpoint, req.PublicKey)
		writeJSON(w, http.StatusOK, map[string]any{"peer_public_key": bundle.PublicKey, "peer_endpoint": bundle.Endpoint, "tunnel_config": bundle, "exit_notified": notified, "exit_notify_error": notifyErr})
		return
	}
	var paired model.PairingCode
	err := s.store.Update(func(st *model.AppState) error {
		c, err := chain.Pair(st.PairingCodes, req.Code, req.Endpoint, req.PublicKey)
		if err != nil {
			return err
		}
		paired = c
		for i := range st.PairingCodes {
			if st.PairingCodes[i].Code == req.Code {
				st.PairingCodes[i].Used = true
			}
		}
		name := chainPeerName(req.DisplayName, c.Endpoint)
		upsertChainPeer(st, model.ChainPeer{ID: util.Token(6), Name: name, Endpoint: c.Endpoint, PublicKey: c.PublicKey, OutboundLink: c.OutboundLink, Status: "paired", CreatedAt: time.Now()})
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer_public_key": paired.PublicKey, "peer_endpoint": paired.Endpoint, "tunnel_config": paired})
}


func (s *Server) RemoveChainPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var removedPeer model.ChainPeer
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainPeers[:0]
		removed := false
		for _, peer := range st.ChainPeers {
			if peer.ID == req.ID {
				removedPeer = peer
				removed = true
				continue
			}
			next = append(next, peer)
		}
		if !removed {
			return errors.New("chain peer not found")
		}
		st.ChainPeers = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	selfEndpoint := chainSelfEndpoint(st)
	notified, notifyErr := notifyRemoteUnpaired(removedPeer.Endpoint, removedPeer.Endpoint, selfEndpoint, "", "")
	resp := publicState(st)
	out := withPanelURL(resp)
	out["exit_notified"] = notified
	if notifyErr != "" {
		out["exit_notify_error"] = notifyErr
	}
	writeJSON(w, http.StatusOK, out)
}


func (s *Server) RemoveChainClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var removed model.ChainClient
	err := s.store.Update(func(st *model.AppState) error {
		next := st.ChainClients[:0]
		found := false
		for _, client := range st.ChainClients {
			if client.ID == req.ID {
				removed = client
				found = true
				continue
			}
			next = append(next, client)
		}
		if !found {
			return errors.New("chain client not found")
		}
		st.ChainClients = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	notified, notifyErr := notifyEntryUnpaired(removed, chainSelfEndpoint(st))
	resp := publicState(st)
	out := withPanelURL(resp)
	out["entry_notified"] = notified
	if notifyErr != "" {
		out["entry_notify_error"] = notifyErr
	}
	writeJSON(w, http.StatusOK, out)
}


func (s *Server) SetChainAccepting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Accepting bool `json:"accepting"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	err := s.store.Update(func(st *model.AppState) error {
		st.ChainPairingDisabled = !req.Accepting
		if st.ChainPairingDisabled {
			st.PairingCodes = nil
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, publicState(s.store.Snapshot()))
}

func upsertChainPeer(st *model.AppState, peer model.ChainPeer) {
	for i := range st.ChainPeers {
		if st.ChainPeers[i].Endpoint == peer.Endpoint || (peer.OutboundLink != "" && st.ChainPeers[i].OutboundLink == peer.OutboundLink) {
			if st.ChainPeers[i].ID != "" {
				peer.ID = st.ChainPeers[i].ID
			}
			if !st.ChainPeers[i].CreatedAt.IsZero() {
				peer.CreatedAt = st.ChainPeers[i].CreatedAt
			}
			st.ChainPeers[i] = peer
			return
		}
	}
	st.ChainPeers = append(st.ChainPeers, peer)
}

func chainPeerMatches(peer model.ChainPeer, endpoint, publicKey, outboundLink string) bool {
	return (endpoint != "" && peer.Endpoint == endpoint) ||
		(publicKey != "" && peer.PublicKey == publicKey) ||
		(outboundLink != "" && peer.OutboundLink == outboundLink)
}

func chainClientMatches(client model.ChainClient, endpoint, publicKey string) bool {
	return (endpoint != "" && client.Endpoint == endpoint) ||
		(publicKey != "" && client.PublicKey == publicKey)
}


func upsertChainClient(st *model.AppState, client model.ChainClient) {
	for i := range st.ChainClients {
		if st.ChainClients[i].Endpoint == client.Endpoint || (client.PublicKey != "" && st.ChainClients[i].PublicKey == client.PublicKey) {
			if st.ChainClients[i].ID != "" {
				client.ID = st.ChainClients[i].ID
			}
			if !st.ChainClients[i].CreatedAt.IsZero() {
				client.CreatedAt = st.ChainClients[i].CreatedAt
			}
			st.ChainClients[i] = client
			return
		}
	}
	st.ChainClients = append(st.ChainClients, client)
}

func notifyExitPaired(bundle chain.Bundle, endpoint, publicKey string) (bool, string) {
	u, err := url.Parse(bundle.Endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, "invalid exit endpoint"
	}
	u.Path = "/api/v1/chain/public/paired"
	u.RawQuery = ""
	body, _ := json.Marshal(map[string]string{"code": bundle.Code, "endpoint": endpoint, "public_key": publicKey, "outbound_link": bundle.OutboundLink})
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(u.String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	if resp == nil {
		return false, "empty response"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "exit returned " + resp.Status
	}
	return true, ""
}

func notifyEntryUnpaired(client model.ChainClient, exitEndpoint string) (bool, string) {
	return notifyRemoteUnpaired(client.Endpoint, exitEndpoint, client.Endpoint, client.PublicKey, client.OutboundLink)
}


func notifyRemoteUnpaired(remoteEndpoint, exitEndpoint, entryEndpoint, publicKey, outboundLink string) (bool, string) {
	u, err := url.Parse(remoteEndpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false, "invalid remote endpoint"
	}
	u.Path = "/api/v1/chain/public/unpaired"
	u.RawQuery = ""
	body, _ := json.Marshal(map[string]string{"exit_endpoint": exitEndpoint, "endpoint": entryEndpoint, "public_key": publicKey, "outbound_link": outboundLink})
	httpClient := http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Post(u.String(), "application/json", bytes.NewReader(body))
	if err != nil {
		return false, err.Error()
	}
	if resp == nil {
		return false, "empty response"
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "entry returned " + resp.Status
	}
	return true, ""
}

func chainSelfEndpoint(st model.AppState) string {
	if !st.IPDirect && st.Domain != "" && st.CertReady {
		return "https://" + st.Domain + ":8443"
	}
	if st.Domain != "" {
		return "http://" + st.Domain + ":8088"
	}
	return ""
}

func chainPeerName(displayName, endpoint string) string {
	if displayName != "" && len(displayName) <= 40 && !strings.HasPrefix(displayName, "Exit ENPAIR-") {
		return displayName
	}
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		return "Exit " + u.Hostname()
	}
	if endpoint != "" && len(endpoint) <= 40 {
		return "Exit " + endpoint
	}
	return "Chain exit"
}

func chainClientName(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
		return "Entry " + u.Hostname()
	}
	if endpoint != "" && len(endpoint) <= 40 {
		return "Entry " + endpoint
	}
	return "Entry server"
}

func firstRunningLink(nodes []model.Node) string {
	for _, n := range nodes {
		if n.Status == "running" {
			if n.SubscribeLink != "" {
				return n.SubscribeLink
			}
			if link := subscribe.Link(n); link != "" {
				return link
			}
		}
	}
	return ""
}

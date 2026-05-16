package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"easynode/internal/core/recommender"
	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"
	"easynode/internal/model"
)

func (s *Server) Nodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Snapshot().Nodes)
}

func (s *Server) AddNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	recs := recommender.Recommend(model.Environment{HasIPv4: true, UDPAvailable: true, TLSReady: true})
	var rec *model.Recommendation
	for i := range recs {
		if recs[i].Protocol == req.Protocol {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		writeError(w, http.StatusBadRequest, errors.New("unknown protocol"))
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		for _, n := range st.Nodes {
			if n.Protocol == req.Protocol {
				return errors.New("protocol already added")
			}
		}
		host := configuredHost(st.Domain, st.IPDirect, r)
		node := newNodeFromRecommendation(*rec, host, st.CertReady)
		st.Nodes = append(st.Nodes, node)
		if req.Protocol == "clash" && !hasProtocol(st.Nodes, "shadowsocks") {
			for _, dep := range recs {
				if dep.Protocol == "shadowsocks" {
					st.Nodes = append(st.Nodes, newNodeFromRecommendation(dep, host, st.CertReady))
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, publicState(st))
}

func (s *Server) RemoveNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	err := s.store.Update(func(st *model.AppState) error {
		next := st.Nodes[:0]
		removed := false
		for _, n := range st.Nodes {
			if n.Protocol == req.Protocol {
				removed = true
				continue
			}
			next = append(next, n)
		}
		if !removed {
			return errors.New("protocol not found")
		}
		st.Nodes = next
		return nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, publicState(st))
}

func (s *Server) NodeAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/nodes/"), "/")
	if len(parts) != 2 || parts[1] != "toggle" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	err := s.store.Update(func(st *model.AppState) error {
		for i := range st.Nodes {
			if st.Nodes[i].ID == id {
				if !protocolRunnable(st.Nodes[i].Protocol, st.CertReady) {
					return errors.New(protocolUnavailableReason(st.Nodes[i].Protocol))
				}
				if st.Nodes[i].Status == "running" {
					st.Nodes[i].Status = "stopped"
				} else {
					st.Nodes[i].Status = "running"
				}
				return nil
			}
		}
		return errors.New("node not found")
	})
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	st := s.store.Snapshot()
	_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	_ = singbox.RestartService()
	writeJSON(w, http.StatusOK, st.Nodes)
}

func (s *Server) ensureRunnableNodes() error {
	changed := false
	err := s.store.Update(func(st *model.AppState) error {
		host := configuredHost(st.Domain, st.IPDirect, nil)
		if st.IPDirect && st.Domain == "" && host != "127.0.0.1" {
			st.Domain = host
			changed = true
		}
		for i := range st.Nodes {
			n := &st.Nodes[i]
			n.Host = host
			if n.Protocol == "vless-reality" {
				if n.RealityPrivateKey == "" || n.RealityPublicKey == "" || n.RealityShortID == "" {
					m := singbox.GenerateRealityMaterial()
					if m.PrivateKey != "" && m.PublicKey != "" {
						n.RealityPrivateKey = m.PrivateKey
						n.RealityPublicKey = m.PublicKey
						n.RealityShortID = m.ShortID
						changed = true
					}
				}
				if n.RealityPrivateKey == "" || n.RealityPublicKey == "" {
					if n.Status == "running" {
						changed = true
					}
					n.Status = "stopped"
					n.SubscribeLink = ""
					continue
				}
				n.Status = "running"
				n.SubscribeLink = subscribe.Link(*n)
				continue
			}
			if protocolRunnable(n.Protocol, st.CertReady) {
				if n.Status == "running" {
					n.SubscribeLink = subscribe.Link(*n)
				}
				continue
			}
			if n.Status == "running" {
				n.Status = "stopped"
				changed = true
			}
			n.SubscribeLink = ""
		}
		if hasProtocol(st.Nodes, "clash") && !hasProtocol(st.Nodes, "shadowsocks") {
			for _, rec := range recommender.Recommend(model.Environment{HasIPv4: true}) {
				if rec.Protocol == "shadowsocks" {
					st.Nodes = append(st.Nodes, newNodeFromRecommendation(rec, host, st.CertReady))
					changed = true
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if changed {
		st := s.store.Snapshot()
		_, _ = singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
		_ = singbox.RestartService()
	}
	return nil
}

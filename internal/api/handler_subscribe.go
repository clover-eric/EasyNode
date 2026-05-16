package api

import (
	"errors"
	"net/http"
	"strings"

	"easynode/internal/core/singbox"
	"easynode/internal/core/subscribe"

	qrcode "github.com/skip2/go-qrcode"
)

func (s *Server) Subscribe(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/v1/subscribe/")
	format := ""
	if strings.HasSuffix(key, "/clash") {
		key = strings.TrimSuffix(key, "/clash")
		format = "clash"
	}
	st := s.store.Snapshot()
	if key != st.SubscribeKey {
		http.NotFound(w, r)
		return
	}
	nodes := nodesForRequest(st, r)
	if format == "clash" || strings.EqualFold(r.URL.Query().Get("format"), "clash") || strings.EqualFold(r.URL.Query().Get("target"), "clash") {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", `inline; filename="easynode-clash.yaml"`)
		_, _ = w.Write([]byte(subscribe.Clash(nodes)))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(subscribe.V2rayN(nodes)))
}

func (s *Server) SubscribeQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	st := s.store.Snapshot()
	writeQRCode(w, publicBaseURL(r)+"/api/v1/subscribe/"+st.SubscribeKey)
}

func (s *Server) ClashQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	st := s.store.Snapshot()
	writeQRCode(w, publicBaseURL(r)+"/api/v1/subscribe/"+st.SubscribeKey+"/clash")
}

func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	return scheme + "://" + host
}

func (s *Server) NodeQRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/qrcode/node/")
	st := s.store.Snapshot()
	for _, n := range st.Nodes {
		if n.ID == id {
			link := n.SubscribeLink
			if link == "" && n.Status == "running" {
				link = subscribe.Link(n)
			}
			if link == "" {
				writeError(w, http.StatusBadRequest, errors.New("node link unavailable"))
				return
			}
			writeQRCode(w, link)
			return
		}
	}
	http.NotFound(w, r)
}

func writeQRCode(w http.ResponseWriter, text string) {
	png, err := qrcode.Encode(text, qrcode.Medium, 260)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(png)
}

func (s *Server) SingBoxConfig(w http.ResponseWriter, r *http.Request) {
	st := s.store.Snapshot()
	path, err := singbox.WriteConfig(s.dataDir, st.Nodes, st.ChainPeers, st.CertPath, st.KeyPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.ServeFile(w, r, path)
}

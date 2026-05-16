package detector

import (
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"easynode/internal/model"
)

func Detect(domain string, ipDirect bool) model.Environment {
	host, _ := os.Hostname()
	env := model.Environment{
		Hostname:     host,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Domain:       domain,
		IPDirect:     ipDirect,
		HasIPv4:      true,
		UDPAvailable: udpProbe(),
		TLSReady:     domain != "" && !ipDirect,
	}
	if domain != "" {
		if ips, err := net.LookupIP(domain); err == nil && len(ips) > 0 {
			env.PublicIP = ips[0].String()
		}
	}
	if env.PublicIP == "" && ipDirect {
		env.PublicIP = discoverPublicIP()
	}
	return env
}

func discoverPublicIP() string {
	client := http.Client{Timeout: 2 * time.Second}
	for _, endpoint := range []string{"https://api.ipify.org", "https://ifconfig.me/ip"} {
		resp, err := client.Get(endpoint)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64))
		_ = resp.Body.Close()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	if ips, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range ips {
			ip, ok := addr.(*net.IPNet)
			if !ok || ip.IP == nil || ip.IP.IsLoopback() || ip.IP.IsPrivate() {
				continue
			}
			if v4 := ip.IP.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

func udpProbe() bool {
	conn, err := net.DialTimeout("udp", "1.1.1.1:53", 800*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

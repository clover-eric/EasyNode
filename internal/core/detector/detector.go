package detector

import (
	"net"
	"os"
	"runtime"
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
	if ips, err := net.LookupIP(domain); err == nil && len(ips) > 0 {
		env.PublicIP = ips[0].String()
	}
	return env
}

func udpProbe() bool {
	conn, err := net.DialTimeout("udp", "1.1.1.1:53", 800*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

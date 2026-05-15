package detector

import (
	"net"
	"strconv"
	"time"
)

func LocalPortLatency(port int, network string) *int {
	if network != "tcp" {
		return nil
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 800*time.Millisecond)
	if err != nil {
		return nil
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return &ms
}

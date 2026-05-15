package detector

import (
	"net"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

var mainlandCache struct {
	sync.Mutex
	value     *int
	checkedAt time.Time
}

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

func MainlandLatency() *int {
	mainlandCache.Lock()
	if time.Since(mainlandCache.checkedAt) < 30*time.Second {
		value := mainlandCache.value
		mainlandCache.Unlock()
		return value
	}
	mainlandCache.Unlock()

	targets := []string{
		"www.baidu.com:443",
		"www.qq.com:443",
		"www.aliyun.com:443",
		"114.114.114.114:53",
	}
	results := make(chan int, len(targets))
	for _, target := range targets {
		go func(addr string) {
			if ms, ok := tcpLatency(addr, 1200*time.Millisecond); ok {
				results <- ms
				return
			}
			results <- 0
		}(target)
	}
	var samples []int
	for range targets {
		ms := <-results
		if ms > 0 {
			samples = append(samples, ms)
		}
	}
	if len(samples) == 0 {
		mainlandCache.Lock()
		mainlandCache.value = nil
		mainlandCache.checkedAt = time.Now()
		mainlandCache.Unlock()
		return nil
	}
	sort.Ints(samples)
	ms := samples[len(samples)/2]
	mainlandCache.Lock()
	mainlandCache.value = &ms
	mainlandCache.checkedAt = time.Now()
	mainlandCache.Unlock()
	return &ms
}

func EndpointLatency(endpoint string) *int {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil
	}
	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		port := "80"
		if u.Scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(u.Hostname(), port)
	}
	if ms, ok := tcpLatency(host, 1200*time.Millisecond); ok {
		return &ms
	}
	return nil
}

func tcpLatency(addr string, timeout time.Duration) (int, bool) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, false
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	return ms, true
}

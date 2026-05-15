package detector

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type PurityReport struct {
	IP          string    `json:"ip"`
	Score       int       `json:"score"`
	Level       string    `json:"level"`
	IPType      string    `json:"ip_type"`
	Native      string    `json:"native"`
	Country     string    `json:"country,omitempty"`
	ASN         string    `json:"asn,omitempty"`
	ISP         string    `json:"isp,omitempty"`
	Risks       []string  `json:"risks"`
	UseCases    []UseCase `json:"use_cases"`
	CheckedAt   string    `json:"checked_at"`
	Explanation string    `json:"explanation"`
}

type UseCase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func CheckPurity() PurityReport {
	report := PurityReport{Score: 70, Level: "unknown", IPType: "unknown", Native: "unknown", CheckedAt: time.Now().Format(time.RFC3339), Explanation: "Basic public IP reputation heuristic"}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/?fields=status,query,country,as,isp,hosting,proxy,mobile")
	if err != nil {
		report.Risks = append(report.Risks, "reputation API unreachable")
		return report
	}
	defer resp.Body.Close()
	var data struct {
		Status  string `json:"status"`
		Query   string `json:"query"`
		Country string `json:"country"`
		AS      string `json:"as"`
		ISP     string `json:"isp"`
		Hosting bool   `json:"hosting"`
		Proxy   bool   `json:"proxy"`
		Mobile  bool   `json:"mobile"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.Status != "success" {
		report.Risks = append(report.Risks, "reputation API returned no result")
		return report
	}
	report.IP = data.Query
	report.Country = data.Country
	report.ASN = data.AS
	report.ISP = data.ISP
	score := 88
	if data.Hosting {
		score -= 18
		report.Risks = append(report.Risks, "data center / hosting ASN")
		report.IPType = "datacenter"
	}
	if data.Proxy {
		score -= 35
		report.Risks = append(report.Risks, "proxy/VPN risk flag")
	}
	if data.Mobile {
		score += 4
		report.IPType = "mobile"
	}
	if report.IPType == "unknown" && !data.Hosting && !data.Proxy {
		report.IPType = "residential/isp-like"
	}
	isp := strings.ToLower(data.ISP + " " + data.AS)
	for _, word := range []string{"cloud", "hosting", "server", "data", "vps"} {
		if strings.Contains(isp, word) {
			score -= 8
			report.Risks = append(report.Risks, "provider name looks like hosting")
			report.IPType = "datacenter"
			break
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	report.Score = score
	switch {
	case score >= 80:
		report.Level = "clean"
		report.Native = "likely native"
	case score >= 55:
		report.Level = "medium"
		report.Native = "uncertain"
	default:
		report.Level = "risky"
		report.Native = "likely proxy/broadcast"
	}
	report.UseCases = useCases(score, report.IPType, data.Proxy)
	return report
}

func useCases(score int, ipType string, proxy bool) []UseCase {
	status := func(ok, warn string) string {
		if proxy || score < 55 {
			return "risky"
		}
		if score < 80 {
			return warn
		}
		return ok
	}
	return []UseCase{
		{Name: "AI / ChatGPT", Status: status("good", "usable"), Reason: "Sensitive to proxy and abuse reputation"},
		{Name: "Streaming", Status: status("good", "limited"), Reason: "Datacenter IPs may hit region or proxy checks"},
		{Name: "Account registration", Status: status("good", "caution"), Reason: "New accounts prefer clean ISP-like IPs"},
		{Name: "Daily browsing", Status: "good", Reason: "Most sites tolerate normal VPS traffic"},
	}
}

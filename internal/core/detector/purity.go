package detector

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type PurityReport struct {
	IP          string   `json:"ip"`
	Score       int      `json:"score"`
	Level       string   `json:"level"`
	Country     string   `json:"country,omitempty"`
	ASN         string   `json:"asn,omitempty"`
	ISP         string   `json:"isp,omitempty"`
	Risks       []string `json:"risks"`
	CheckedAt   string   `json:"checked_at"`
	Explanation string   `json:"explanation"`
}

func CheckPurity() PurityReport {
	report := PurityReport{Score: 70, Level: "unknown", CheckedAt: time.Now().Format(time.RFC3339), Explanation: "Basic public IP reputation heuristic"}
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
	}
	if data.Proxy {
		score -= 35
		report.Risks = append(report.Risks, "proxy/VPN risk flag")
	}
	if data.Mobile {
		score += 4
	}
	isp := strings.ToLower(data.ISP + " " + data.AS)
	for _, word := range []string{"cloud", "hosting", "server", "data", "vps"} {
		if strings.Contains(isp, word) {
			score -= 8
			report.Risks = append(report.Risks, "provider name looks like hosting")
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
	case score >= 55:
		report.Level = "medium"
	default:
		report.Level = "risky"
	}
	return report
}

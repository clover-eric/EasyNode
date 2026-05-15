package detector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PurityReport struct {
	IP                 string      `json:"ip"`
	Score              int         `json:"score"`
	Level              string      `json:"level"`
	IPType             string      `json:"ip_type"`
	Native             string      `json:"native"`
	Country            string      `json:"country,omitempty"`
	CountryCode        string      `json:"country_code,omitempty"`
	LocationConfidence string      `json:"location_confidence,omitempty"`
	LocationSources    []GeoSource `json:"location_sources,omitempty"`
	ASN                string      `json:"asn,omitempty"`
	ISP                string      `json:"isp,omitempty"`
	Risks              []string    `json:"risks"`
	UseCases           []UseCase   `json:"use_cases"`
	CheckedAt          string      `json:"checked_at"`
	Explanation        string      `json:"explanation"`
}

type GeoSource struct {
	Source      string `json:"source"`
	IP          string `json:"ip,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	ASN         string `json:"asn,omitempty"`
	ISP         string `json:"isp,omitempty"`
}

type UseCase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func CheckPurity() PurityReport {
	report := PurityReport{Score: 70, Level: "unknown", IPType: "unknown", Native: "unknown", CheckedAt: time.Now().Format(time.RFC3339), Explanation: "Basic public IP reputation heuristic"}
	client := http.Client{Timeout: 2 * time.Second}
	sources := geoSources(client)
	if len(sources) > 0 {
		applyBestGeo(&report, sources)
	}
	resp, err := client.Get("http://ip-api.com/json/?fields=status,query,country,countryCode,as,isp,hosting,proxy,mobile")
	if err != nil {
		report.Risks = append(report.Risks, "reputation API unreachable")
		return report
	}
	defer resp.Body.Close()
	var data struct {
		Status      string `json:"status"`
		Query       string `json:"query"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		AS          string `json:"as"`
		ISP         string `json:"isp"`
		Hosting     bool   `json:"hosting"`
		Proxy       bool   `json:"proxy"`
		Mobile      bool   `json:"mobile"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.Status != "success" {
		report.Risks = append(report.Risks, "reputation API returned no result")
		return report
	}
	if report.IP == "" {
		report.IP = data.Query
	}
	if report.Country == "" {
		report.Country = data.Country
		report.CountryCode = data.CountryCode
	}
	if report.ASN == "" {
		report.ASN = data.AS
	}
	if report.ISP == "" {
		report.ISP = data.ISP
	}
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

func geoSources(client http.Client) []GeoSource {
	var out []GeoSource
	if s, ok := fromIPWho(client); ok {
		out = append(out, s)
	}
	if s, ok := fromIPAPICo(client); ok {
		out = append(out, s)
	}
	if s, ok := fromIPInfo(client); ok {
		out = append(out, s)
	}
	if s, ok := fromIPAPI(client); ok {
		out = append(out, s)
	}
	return out
}

func fromIPWho(client http.Client) (GeoSource, bool) {
	resp, err := client.Get("https://ipwho.is/")
	if err != nil {
		return GeoSource{}, false
	}
	defer resp.Body.Close()
	var data struct {
		Success     bool   `json:"success"`
		IP          string `json:"ip"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Connection  struct {
			ASN int    `json:"asn"`
			ISP string `json:"isp"`
			Org string `json:"org"`
		} `json:"connection"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || !data.Success {
		return GeoSource{}, false
	}
	return GeoSource{Source: "ipwho.is", IP: data.IP, Country: data.Country, CountryCode: strings.ToUpper(data.CountryCode), ASN: asnText(data.Connection.ASN, data.Connection.Org), ISP: firstNonEmpty(data.Connection.ISP, data.Connection.Org)}, true
}

func fromIPAPICo(client http.Client) (GeoSource, bool) {
	resp, err := client.Get("https://ipapi.co/json/")
	if err != nil {
		return GeoSource{}, false
	}
	defer resp.Body.Close()
	var data struct {
		IP          string `json:"ip"`
		CountryName string `json:"country_name"`
		CountryCode string `json:"country"`
		ASN         string `json:"asn"`
		Org         string `json:"org"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.IP == "" {
		return GeoSource{}, false
	}
	return GeoSource{Source: "ipapi.co", IP: data.IP, Country: data.CountryName, CountryCode: strings.ToUpper(data.CountryCode), ASN: data.ASN, ISP: data.Org}, true
}

func fromIPInfo(client http.Client) (GeoSource, bool) {
	resp, err := client.Get("https://ipinfo.io/json")
	if err != nil {
		return GeoSource{}, false
	}
	defer resp.Body.Close()
	var data struct {
		IP      string `json:"ip"`
		Country string `json:"country"`
		Org     string `json:"org"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.IP == "" {
		return GeoSource{}, false
	}
	return GeoSource{Source: "ipinfo.io", IP: data.IP, Country: countryByCode(data.Country), CountryCode: strings.ToUpper(data.Country), ASN: data.Org, ISP: data.Org}, true
}

func fromIPAPI(client http.Client) (GeoSource, bool) {
	resp, err := client.Get("http://ip-api.com/json/?fields=status,query,country,countryCode,as,isp")
	if err != nil {
		return GeoSource{}, false
	}
	defer resp.Body.Close()
	var data struct {
		Status      string `json:"status"`
		Query       string `json:"query"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		AS          string `json:"as"`
		ISP         string `json:"isp"`
	}
	if json.NewDecoder(resp.Body).Decode(&data) != nil || data.Status != "success" {
		return GeoSource{}, false
	}
	return GeoSource{Source: "ip-api.com", IP: data.Query, Country: data.Country, CountryCode: strings.ToUpper(data.CountryCode), ASN: data.AS, ISP: data.ISP}, true
}

func applyBestGeo(report *PurityReport, sources []GeoSource) {
	report.LocationSources = sources
	counts := map[string]int{}
	for _, s := range sources {
		if s.CountryCode != "" {
			counts[s.CountryCode]++
		}
	}
	bestCode := ""
	bestCount := 0
	for code, n := range counts {
		if n > bestCount {
			bestCode = code
			bestCount = n
		}
	}
	selected := sources[0]
	if bestCount > 1 {
		for _, s := range sources {
			if s.CountryCode == bestCode {
				selected = s
				break
			}
		}
		report.LocationConfidence = "multi-source match"
	} else if len(sources) > 1 {
		report.LocationConfidence = "source conflict"
		report.Risks = append(report.Risks, "location database conflict")
	} else {
		report.LocationConfidence = "single source"
	}
	report.IP = selected.IP
	report.Country = selected.Country
	report.CountryCode = selected.CountryCode
	report.ASN = selected.ASN
	report.ISP = selected.ISP
}

func asnText(asn int, org string) string {
	if asn == 0 {
		return org
	}
	if org == "" {
		return fmt.Sprintf("AS%d", asn)
	}
	return fmt.Sprintf("AS%d %s", asn, org)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func countryByCode(code string) string {
	switch strings.ToUpper(code) {
	case "HK":
		return "Hong Kong"
	case "SG":
		return "Singapore"
	case "US":
		return "United States"
	case "JP":
		return "Japan"
	case "TW":
		return "Taiwan"
	case "KR":
		return "South Korea"
	case "GB":
		return "United Kingdom"
	case "DE":
		return "Germany"
	case "FR":
		return "France"
	case "CA":
		return "Canada"
	case "AU":
		return "Australia"
	case "NL":
		return "Netherlands"
	case "CN":
		return "China"
	default:
		return strings.ToUpper(code)
	}
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

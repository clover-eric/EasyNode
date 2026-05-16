package cert

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const WebrootPath = "/var/lib/easynode/acme-challenge"

type Result struct {
	Ready    bool
	CertPath string
	KeyPath  string
	Output   string
}

func Ensure(domain string) (Result, error) {
	if domain == "" {
		return Result{}, fmt.Errorf("domain required")
	}
	if _, err := exec.LookPath("certbot"); err != nil {
		return Result{}, fmt.Errorf("certbot not installed")
	}
	_ = os.MkdirAll(WebrootPath, 0755)
	res := Result{
		CertPath: "/etc/letsencrypt/live/" + domain + "/fullchain.pem",
		KeyPath:  "/etc/letsencrypt/live/" + domain + "/privkey.pem",
	}
	if fileExists(res.CertPath) && fileExists(res.KeyPath) {
		res.Ready = true
		return res, nil
	}
	out, err := certbotWebroot(domain)
	if err != nil {
		out, err = certbotStandalone(domain)
	}
	res.Output = string(out)
	if err != nil {
		return res, fmt.Errorf("certificate request failed: %w: %s", err, string(out))
	}
	res.Ready = true
	return res, nil
}

func certbotWebroot(domain string) ([]byte, error) {
	return exec.Command("certbot",
		"certonly", "--webroot", "--non-interactive", "--agree-tos",
		"--register-unsafely-without-email",
		"--keep-until-expiring",
		"-w", WebrootPath,
		"-d", domain,
	).CombinedOutput()
}

func certbotStandalone(domain string) ([]byte, error) {
	return exec.Command("certbot",
		"certonly", "--standalone", "--non-interactive", "--agree-tos",
		"--register-unsafely-without-email",
		"--preferred-challenges", "http",
		"--keep-until-expiring",
		"-d", domain,
	).CombinedOutput()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func ChallengeDir() string {
	return filepath.Join(WebrootPath, ".well-known", "acme-challenge")
}

package cert

import (
	"fmt"
	"os/exec"
)

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
	args := []string{
		"certonly", "--standalone", "--non-interactive", "--agree-tos",
		"--register-unsafely-without-email",
		"--preferred-challenges", "http",
		"--keep-until-expiring",
		"-d", domain,
	}
	out, err := exec.Command("certbot", args...).CombinedOutput()
	res := Result{
		Ready:    err == nil,
		CertPath: "/etc/letsencrypt/live/" + domain + "/fullchain.pem",
		KeyPath:  "/etc/letsencrypt/live/" + domain + "/privkey.pem",
		Output:   string(out),
	}
	if err != nil {
		return res, fmt.Errorf("certificate request failed: %w: %s", err, string(out))
	}
	return res, nil
}

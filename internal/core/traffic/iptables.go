package traffic

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var dptRe = regexp.MustCompile(`dpt:(\d+)`)
var sptRe = regexp.MustCompile(`spt:(\d+)`)

func PortBytes() map[int]int64 {
	out := map[int]int64{}
	for _, args := range [][]string{{"-L", "EASYNODE_TRAFFIC_IN", "-v", "-x", "-n"}, {"-L", "EASYNODE_TRAFFIC_OUT", "-v", "-x", "-n"}} {
		b, err := exec.Command("iptables", args...).CombinedOutput()
		if err != nil {
			continue
		}
		parse(out, string(b))
	}
	return out
}

func parse(out map[int]int64, text string) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		bytes, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		m := dptRe.FindStringSubmatch(line)
		if len(m) != 2 {
			m = sptRe.FindStringSubmatch(line)
		}
		if len(m) != 2 {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err == nil {
			out[port] += bytes
		}
	}
}

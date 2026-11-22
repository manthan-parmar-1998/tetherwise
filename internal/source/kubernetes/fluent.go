package kubernetes

import (
	"bufio"
	"strings"
)

type fluentOutput struct {
	name string
	host string
}

// fluentBitHosts reads Fluent Bit OUTPUT sections. The config is a small
// line-oriented format (repeated [OUTPUT] blocks) with no stable Go parser,
// so this walks those blocks and keeps Name and Host. YAML and JSON
// manifests are decoded by sigs.k8s.io/yaml before this runs.
func fluentBitHosts(text string) []string {
	if !strings.Contains(strings.ToUpper(text), "[OUTPUT]") {
		return nil
	}
	var hosts []string
	var section string
	var current fluentOutput
	flush := func() {
		if strings.EqualFold(section, "OUTPUT") && searchPlugin(current.name) && current.host != "" {
			hosts = append(hosts, current.host)
		}
		current = fluentOutput{}
	}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			section = strings.Trim(line, "[]")
			continue
		}
		if !strings.EqualFold(section, "OUTPUT") {
			continue
		}
		key, val, ok := splitKV(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "name":
			current.name = val
		case "host":
			current.host = val
		}
	}
	flush()
	return hosts
}

func splitKV(line string) (string, string, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], strings.Join(fields[1:], " "), true
}

func searchPlugin(name string) bool {
	switch strings.ToLower(name) {
	case "es", "opensearch", "elasticsearch":
		return true
	default:
		return false
	}
}

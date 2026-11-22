package kubernetes

import (
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// vectorEndpoints reads Vector sink endpoints from a TOML document.
// Documents that are not Vector config return no endpoints.
func vectorEndpoints(text string) []string {
	if !strings.Contains(text, "sinks") {
		return nil
	}
	var root map[string]any
	if err := toml.Unmarshal([]byte(text), &root); err != nil {
		return nil
	}
	sinks, _ := root["sinks"].(map[string]any)
	var hosts []string
	for _, raw := range sinks {
		sink, _ := raw.(map[string]any)
		typ, _ := sink["type"].(string)
		switch strings.ToLower(typ) {
		case "elasticsearch", "opensearch":
		default:
			continue
		}
		hosts = append(hosts, endpointValues(sink["endpoints"])...)
		hosts = append(hosts, endpointValues(sink["endpoint"])...)
	}
	return hosts
}

func endpointValues(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

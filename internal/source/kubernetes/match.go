package kubernetes

import (
	"net/url"
	"strings"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

type knownMatch struct {
	id         string
	confidence model.Confidence
}

var endpointAttrs = []string{
	"endpoint", "domain_endpoint", "address", "bootstrap_brokers", "bootstrap_brokers_tls", "dns_name",
}

// matchKnown finds resources whose endpoint or name appears in value.
// An exact endpoint match is high confidence. A DNS-label match on the
// resource name is medium.
func matchKnown(value string, known []model.Resource) []knownMatch {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var out []knownMatch
	for _, res := range known {
		conf, ok := confidenceFor(value, res)
		if !ok {
			continue
		}
		out = append(out, knownMatch{id: res.ID, confidence: conf})
	}
	return out
}

func confidenceFor(value string, res model.Resource) (model.Confidence, bool) {
	for _, key := range endpointAttrs {
		ep := res.Attrs[key]
		if len(ep) < 8 {
			continue
		}
		if strings.Contains(value, ep) {
			return model.ConfidenceHigh, true
		}
	}
	switch res.Kind {
	case "opensearch:domain", "rds:instance", "rds:cluster", "msk:cluster":
		if nameInHost(value, res.Name) {
			return model.ConfidenceMedium, true
		}
	}
	return "", false
}

func nameInHost(value, name string) bool {
	if len(name) < 3 {
		return false
	}
	host := value
	if parsed, err := url.Parse(value); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Split(host, "/")[0]
	host = strings.Split(host, ":")[0]
	for _, label := range strings.Split(host, ".") {
		if strings.EqualFold(label, name) {
			return true
		}
	}
	return false
}

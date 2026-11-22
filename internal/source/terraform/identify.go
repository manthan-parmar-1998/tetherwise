package terraform

import (
	"strings"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

type identity struct {
	provider string
	kind     string
	keys     []string
}

// knownTypes maps a Terraform type to a cloud-neutral kind and the
// attribute names that hold its name, in preference order.
var knownTypes = map[string]identity{
	"aws_opensearch_domain":                  {provider: "aws", kind: "opensearch:domain", keys: []string{"domain_name", "name"}},
	"aws_elasticsearch_domain":               {provider: "aws", kind: "opensearch:domain", keys: []string{"domain_name", "name"}},
	"aws_db_instance":                        {provider: "aws", kind: "rds:instance", keys: []string{"identifier", "id"}},
	"aws_db_cluster":                         {provider: "aws", kind: "rds:cluster", keys: []string{"cluster_identifier", "id"}},
	"aws_msk_cluster":                        {provider: "aws", kind: "msk:cluster", keys: []string{"cluster_name", "name"}},
	"aws_security_group":                     {provider: "aws", kind: "ec2:security-group", keys: []string{"name", "id"}},
	"aws_cloudwatch_log_group":               {provider: "aws", kind: "logs:log-group", keys: []string{"name"}},
	"aws_cloudwatch_log_subscription_filter": {provider: "aws", kind: "logs:subscription-filter", keys: []string{"name"}},
	"aws_kinesis_firehose_delivery_stream":   {provider: "aws", kind: "firehose:delivery-stream", keys: []string{"name"}},
	"aws_elasticache_cluster":                {provider: "aws", kind: "elasticache:cluster", keys: []string{"cluster_id", "id"}},
	"kubernetes_deployment":                  {provider: "k8s", kind: "deployment", keys: []string{"name"}},
	"kubernetes_stateful_set":                {provider: "k8s", kind: "statefulset", keys: []string{"name"}},
	"kubernetes_daemonset":                   {provider: "k8s", kind: "daemonset", keys: []string{"name"}},
}

var copiedAttrs = []string{
	"endpoint", "domain_endpoint", "domain_name", "identifier", "cluster_name",
	"cluster_id", "engine", "engine_version", "arn", "id", "name", "dns_name",
	"bootstrap_brokers", "bootstrap_brokers_tls", "address", "filter_pattern",
	"retention_in_days", "log_group_name", "port", "publicly_accessible",
}

func identify(typ, address string, values map[string]any, hidden map[string]bool) (model.Resource, bool) {
	if values == nil {
		values = map[string]any{}
	}
	spec, ok := knownTypes[typ]
	if !ok {
		spec, ok = genericIdentity(typ)
		if !ok {
			return model.Resource{}, false
		}
	}
	name := firstString(values, spec.keys...)
	if spec.provider == "k8s" {
		ns := firstString(values, "namespace")
		if ns == "" {
			ns = "default"
		}
		if name == "" {
			return model.Resource{}, false
		}
		name = ns + "/" + name
	}
	if name == "" {
		return model.Resource{}, false
	}
	id, err := model.ResourceID(spec.provider, spec.kind, name)
	if err != nil {
		return model.Resource{}, false
	}
	team, owner := tagsOf(values)
	attrs := map[string]string{}
	if address != "" {
		attrs["terraform_address"] = address
	}
	if typ != "" {
		attrs["terraform_type"] = typ
	}
	for _, key := range copiedAttrs {
		if hidden[key] || sensitiveKey(key) {
			continue
		}
		if text, ok := scalarString(values[key]); ok {
			attrs[key] = text
		}
	}
	for key, ep := range nestedEndpoints(values) {
		if hidden[key] || sensitiveKey(key) {
			continue
		}
		attrs[key] = ep
	}
	if groups := securityGroupRefs(values); len(groups) > 0 {
		attrs["security_groups"] = strings.Join(groups, ",")
	}
	res := model.Resource{
		ID: id, Provider: spec.provider, Kind: spec.kind, Name: name,
		Team: team, Owner: owner, Attrs: attrs,
	}
	if err := res.Validate(); err != nil {
		return model.Resource{}, false
	}
	return res, true
}

func genericIdentity(typ string) (identity, bool) {
	switch {
	case strings.HasPrefix(typ, "aws_"):
		kind := strings.ReplaceAll(strings.TrimPrefix(typ, "aws_"), "_", ":")
		if !kindOK(kind) {
			return identity{}, false
		}
		return identity{provider: "aws", kind: kind, keys: []string{"name", "id"}}, true
	case strings.HasPrefix(typ, "kubernetes_"):
		kind := strings.ReplaceAll(strings.TrimPrefix(typ, "kubernetes_"), "_", "-")
		if !kindOK(kind) {
			return identity{}, false
		}
		return identity{provider: "k8s", kind: kind, keys: []string{"name"}}, true
	default:
		return identity{}, false
	}
}

func kindOK(kind string) bool {
	_, err := model.ResourceID("aws", kind, "placeholder")
	return err == nil
}

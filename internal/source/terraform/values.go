package terraform

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := scalarString(values[key]); ok {
			return text
		}
	}
	return ""
}

func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "", false
		}
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return "", false
		}
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10), true
		}
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case json.Number:
		if t == "" {
			return "", false
		}
		return t.String(), true
	default:
		return "", false
	}
}

func tagsOf(values map[string]any) (team, owner string) {
	for _, key := range []string{"tags", "tags_all"} {
		raw, ok := values[key].(map[string]any)
		if !ok {
			continue
		}
		if team == "" {
			team = firstString(raw, "Team", "team")
		}
		if owner == "" {
			owner = firstString(raw, "Owner", "owner")
		}
	}
	return team, owner
}

func sensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, word := range []string{"password", "secret", "token", "private_key", "access_key", "credential"} {
		if strings.Contains(k, word) {
			return true
		}
	}
	return false
}

func sensitiveKeys(raw json.RawMessage) map[string]bool {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	m, ok := decoded.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]bool{}
	for k, v := range m {
		if b, ok := v.(bool); ok && b {
			out[k] = true
		}
	}
	return out
}

func nestedEndpoints(values map[string]any) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"endpoint", "domain_endpoint", "address"} {
		raw, ok := values[key].(map[string]any)
		if !ok {
			continue
		}
		if addr, ok := scalarString(raw["address"]); ok {
			out[key] = addr
		}
	}
	return out
}

func endpointValue(values map[string]any, key string) string {
	if text, ok := scalarString(values[key]); ok {
		return text
	}
	if found := nestedEndpoints(values); found[key] != "" {
		return found[key]
	}
	return ""
}

func securityGroupRefs(values map[string]any) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, key := range []string{"vpc_security_group_ids", "security_groups", "security_group_ids"} {
		for _, item := range stringsOf(values[key]) {
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func stringsOf(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func collectStrings(values map[string]any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if t != "" {
				out = append(out, t)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if sensitiveKey(k) {
					continue
				}
				walk(t[k])
			}
		}
	}
	walk(values)
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func changedAttrs(before, after, beforeSensitive, afterSensitive any) []string {
	hidden := map[string]bool{}
	for _, raw := range []any{beforeSensitive, afterSensitive} {
		for k, v := range asMap(raw) {
			if b, ok := v.(bool); ok && b {
				hidden[k] = true
			}
		}
	}
	b := asMap(before)
	a := asMap(after)
	keys := map[string]struct{}{}
	for k := range b {
		keys[k] = struct{}{}
	}
	for k := range a {
		keys[k] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)
	var out []string
	for _, k := range ordered {
		if hidden[k] || sensitiveKey(k) {
			continue
		}
		if jsonEqual(b[k], a[k]) {
			continue
		}
		out = append(out, k)
	}
	return out
}

func jsonEqual(a, b any) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

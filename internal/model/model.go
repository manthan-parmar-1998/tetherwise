package model

import (
	"regexp"
	"sort"
	"strings"
)

// Confidence is how a dependency was established.
type Confidence string

// Confidence levels recorded on a dependency.
const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Rank orders confidence so a merge can keep the stronger observation.
// Unknown values rank below low.
func (c Confidence) Rank() int {
	switch c {
	case ConfidenceHigh:
		return 3
	case ConfidenceMedium:
		return 2
	case ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// Valid reports whether c is one of the three known levels.
func (c Confidence) Valid() bool {
	return c.Rank() > 0
}

// Dependency kinds. v0.1 uses this closed vocabulary.
const (
	KindWritesLogsTo = "writes-logs-to"
	KindReadsFrom    = "reads-from"
	KindConnectsTo   = "connects-to"
)

// Dependency sources recorded on an edge.
const (
	SourceTerraform = "terraform"
	SourceK8sEnv    = "k8s-env"
	SourceFluentBit = "fluentbit"
	SourceVector    = "vector"
	SourceEdgesFile = "edges-file"
)

// Direction says which side of a threshold is the safe side.
type Direction string

const (
	// DirectionLowerBetter means headroom is threshold minus current
	// (CPU, memory pressure, connections, disk used).
	DirectionLowerBetter Direction = "lower-better"
	// DirectionHigherBetter means headroom is current minus threshold
	// (free storage, free memory).
	DirectionHigherBetter Direction = "higher-better"
)

// Action is the Terraform-style change applied to a resource.
type Action string

// Actions a plan can apply to a resource.
const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionReplace Action = "replace"
)

const (
	providerPattern = `^[a-z0-9]+$`
	kindPattern     = `^[a-z0-9]+(?::[a-z0-9-]+)*$`
	namePattern     = `^[A-Za-z0-9_./:-]+$`
	slugPattern     = `^[a-z0-9]+(?:-[a-z0-9]+)*$`
)

var (
	providerRE = regexp.MustCompile(providerPattern)
	kindRE     = regexp.MustCompile(kindPattern)
	nameRE     = regexp.MustCompile(namePattern)
	slugRE     = regexp.MustCompile(slugPattern)
)

// ValidationError is a rejected domain value.
type ValidationError struct {
	Subject string
	Field   string
	Problem string
}

func (e *ValidationError) Error() string {
	return e.Subject + " " + e.Field + ": " + e.Problem
}

func invalid(subject, field, problem string) error {
	return &ValidationError{Subject: subject, Field: field, Problem: problem}
}

// Resource is one cloud or cluster object.
//
// ID is cloud-neutral: provider, kind, and name joined as
// "aws:opensearch:domain/search-lab" or "k8s:deployment/atlas/atlas-indexer".
// Kind may itself contain colons. Name may contain slashes.
type Resource struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Provider string            `json:"provider"`
	Name     string            `json:"name"`
	Owner    string            `json:"owner,omitempty"`
	Team     string            `json:"team,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

// ResourceID builds an ID from its three parts.
func ResourceID(provider, kind, name string) (string, error) {
	if !providerRE.MatchString(provider) {
		return "", invalid("resource", "provider", "must match "+providerPattern)
	}
	if !kindRE.MatchString(kind) {
		return "", invalid("resource", "kind", "must match "+kindPattern)
	}
	if !nameRE.MatchString(name) {
		return "", invalid("resource", "name", "must match "+namePattern)
	}
	return provider + ":" + kind + "/" + name, nil
}

// ParseID splits an ID into provider, kind, and name.
func ParseID(id string) (provider, kind, name string, err error) {
	colon := strings.IndexByte(id, ':')
	slash := strings.IndexByte(id, '/')
	if colon <= 0 || slash <= colon+1 || slash >= len(id)-1 {
		return "", "", "", invalid("resource", "id", "must look like provider:kind/name")
	}
	provider = id[:colon]
	kind = id[colon+1 : slash]
	name = id[slash+1:]
	if _, err = ResourceID(provider, kind, name); err != nil {
		return "", "", "", invalid("resource", "id", "must look like provider:kind/name")
	}
	if provider+":"+kind+"/"+name != id {
		return "", "", "", invalid("resource", "id", "must look like provider:kind/name")
	}
	return provider, kind, name, nil
}

// ResourceFromID builds a resource that carries only its identity.
func ResourceFromID(id string) (Resource, error) {
	provider, kind, name, err := ParseID(id)
	if err != nil {
		return Resource{}, err
	}
	return Resource{ID: id, Provider: provider, Kind: kind, Name: name}, nil
}

// Validate checks identity fields and attribute keys.
func (r Resource) Validate() error {
	provider, kind, name, err := ParseID(r.ID)
	if err != nil {
		return err
	}
	if r.Provider != provider {
		return invalid("resource", "provider", "does not match id")
	}
	if r.Kind != kind {
		return invalid("resource", "kind", "does not match id")
	}
	if r.Name != name {
		return invalid("resource", "name", "does not match id")
	}
	if strings.ContainsAny(r.Owner, "\r\n") || strings.ContainsAny(r.Team, "\r\n") {
		return invalid("resource", "owner", "must be a single line")
	}
	for k := range r.Attrs {
		if k == "" || strings.ContainsAny(k, " \r\n") {
			return invalid("resource", "attrs", "keys must be non-empty tokens")
		}
	}
	return nil
}

// Dependency is a directed edge from a consumer to a resource it uses.
type Dependency struct {
	From       string     `json:"from"`
	To         string     `json:"to"`
	Kind       string     `json:"kind"`
	Source     string     `json:"source"`
	Sources    []string   `json:"sources,omitempty"`
	Confidence Confidence `json:"confidence"`
}

// Validate checks endpoints, kind, source, and confidence.
func (d Dependency) Validate() error {
	if _, _, _, err := ParseID(d.From); err != nil {
		return invalid("dependency", "from", "must be a resource id")
	}
	if _, _, _, err := ParseID(d.To); err != nil {
		return invalid("dependency", "to", "must be a resource id")
	}
	if d.From == d.To {
		return invalid("dependency", "to", "must differ from from")
	}
	switch d.Kind {
	case KindWritesLogsTo, KindReadsFrom, KindConnectsTo:
	default:
		return invalid("dependency", "kind", "must be writes-logs-to, reads-from, or connects-to")
	}
	if !slugRE.MatchString(d.Source) {
		return invalid("dependency", "source", "must be a lowercase slug")
	}
	if !d.Confidence.Valid() {
		return invalid("dependency", "confidence", "must be high, medium, or low")
	}
	if len(d.Sources) > 0 && !contains(d.Sources, d.Source) {
		return invalid("dependency", "sources", "must include source")
	}
	for _, s := range d.Sources {
		if !slugRE.MatchString(s) {
			return invalid("dependency", "sources", "must be lowercase slugs")
		}
	}
	return nil
}

// Limit is one metric sample compared with a threshold.
// Headroom is positive when the resource is inside the limit.
type Limit struct {
	Resource  string    `json:"resource"`
	Metric    string    `json:"metric"`
	Threshold float64   `json:"threshold"`
	Current   float64   `json:"current"`
	Headroom  float64   `json:"headroom"`
	Unit      string    `json:"unit,omitempty"`
	Direction Direction `json:"direction"`
}

// ComputeHeadroom returns threshold-current or current-threshold.
func ComputeHeadroom(direction Direction, threshold, current float64) (float64, error) {
	switch direction {
	case DirectionLowerBetter:
		return threshold - current, nil
	case DirectionHigherBetter:
		return current - threshold, nil
	default:
		return 0, invalid("limit", "direction", "must be lower-better or higher-better")
	}
}

// NewLimit builds a limit and fills headroom from the direction.
func NewLimit(resource, metric string, threshold, current float64, unit string, direction Direction) (Limit, error) {
	headroom, err := ComputeHeadroom(direction, threshold, current)
	if err != nil {
		return Limit{}, err
	}
	limit := Limit{
		Resource:  resource,
		Metric:    metric,
		Threshold: threshold,
		Current:   current,
		Headroom:  headroom,
		Unit:      unit,
		Direction: direction,
	}
	if err := limit.Validate(); err != nil {
		return Limit{}, err
	}
	return limit, nil
}

// Validate checks the resource id, metric name, direction, and headroom.
func (l Limit) Validate() error {
	if _, _, _, err := ParseID(l.Resource); err != nil {
		return invalid("limit", "resource", "must be a resource id")
	}
	if !slugRE.MatchString(strings.ToLower(l.Metric)) && !metricRE.MatchString(l.Metric) {
		return invalid("limit", "metric", "must be a metric name")
	}
	switch l.Direction {
	case DirectionLowerBetter, DirectionHigherBetter:
	default:
		return invalid("limit", "direction", "must be lower-better or higher-better")
	}
	want, err := ComputeHeadroom(l.Direction, l.Threshold, l.Current)
	if err != nil {
		return err
	}
	if !sameFloat(want, l.Headroom) {
		return invalid("limit", "headroom", "must equal the value implied by direction, threshold, and current")
	}
	return nil
}

// metricRE allows CloudWatch names such as JVMMemoryPressure and ClusterStatus.red.
var metricRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.]*$`)

// Change is one planned mutation of a resource.
type Change struct {
	Resource     string   `json:"resource"`
	Action       Action   `json:"action"`
	ChangedAttrs []string `json:"changed_attrs,omitempty"`
}

// Validate checks the resource id, action, and attribute names.
func (c Change) Validate() error {
	if _, _, _, err := ParseID(c.Resource); err != nil {
		return invalid("change", "resource", "must be a resource id")
	}
	switch c.Action {
	case ActionCreate, ActionUpdate, ActionDelete, ActionReplace:
	default:
		return invalid("change", "action", "must be create, update, delete, or replace")
	}
	seen := map[string]struct{}{}
	for _, attr := range c.ChangedAttrs {
		if attr == "" || strings.ContainsAny(attr, " \r\n") {
			return invalid("change", "changed_attrs", "must be non-empty tokens")
		}
		if _, ok := seen[attr]; ok {
			return invalid("change", "changed_attrs", "must not contain duplicates")
		}
		seen[attr] = struct{}{}
	}
	return nil
}

// SortResources orders resources by ID.
func SortResources(rs []Resource) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].ID < rs[j].ID })
}

// SortDependencies orders edges by from, to, then kind.
func SortDependencies(ds []Dependency) {
	sort.Slice(ds, func(i, j int) bool {
		if ds[i].From != ds[j].From {
			return ds[i].From < ds[j].From
		}
		if ds[i].To != ds[j].To {
			return ds[i].To < ds[j].To
		}
		return ds[i].Kind < ds[j].Kind
	})
}

// SortLimits orders limits by resource, then metric.
func SortLimits(ls []Limit) {
	sort.Slice(ls, func(i, j int) bool {
		if ls[i].Resource != ls[j].Resource {
			return ls[i].Resource < ls[j].Resource
		}
		return ls[i].Metric < ls[j].Metric
	})
}

// SortChanges orders changes by resource, then action.
func SortChanges(cs []Change) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Resource != cs[j].Resource {
			return cs[i].Resource < cs[j].Resource
		}
		return cs[i].Action < cs[j].Action
	})
}

// SortStrings returns a sorted copy of unique values.
func SortStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func sameFloat(a, b float64) bool {
	const eps = 1e-6
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

// TeamKey is the case-insensitive identity of a team or owner string.
func TeamKey(team string) string {
	return strings.ToLower(strings.TrimSpace(team))
}

// DisplayTeam returns the first non-empty label, preferring team over owner.
func DisplayTeam(team, owner string) string {
	if strings.TrimSpace(team) != "" {
		return strings.TrimSpace(team)
	}
	return strings.TrimSpace(owner)
}

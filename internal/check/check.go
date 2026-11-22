package check

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/manthan-parmar-1998/tetherwise/internal/graph"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// SchemaVersion is the report.json schema this result serializes as.
const SchemaVersion = "v0.1"

// DefaultTightFraction is how close headroom must be to zero, relative to
// the threshold, before a limit counts as tight. Headroom at or below zero
// is always tight.
const DefaultTightFraction = 0.15

// Verdict is the exposure conclusion.
type Verdict string

// Verdicts, from no shared impact to a shared resource at its limit.
const (
	VerdictInfo Verdict = "info"
	VerdictWarn Verdict = "warn"
	VerdictHigh Verdict = "high-exposure"
)

// Options adjusts the tight-headroom comparison.
type Options struct {
	TightFraction float64
}

func (o Options) fraction() float64 {
	if o.TightFraction <= 0 {
		return DefaultTightFraction
	}
	return o.TightFraction
}

// Result is the exposure check.
type Result struct {
	SchemaVersion string         `json:"schema_version"`
	Verdict       Verdict        `json:"verdict"`
	Reasons       []string       `json:"reasons"`
	Changes       []model.Change `json:"changes"`
	Affected      []Affected     `json:"affected"`
	Shared        []Shared       `json:"shared"`
	Teams         []string       `json:"teams"`
	Owners        []string       `json:"owners"`
	Coverage      Coverage       `json:"coverage"`
}

// Affected is one service that depends on a shared resource in the blast radius.
type Affected struct {
	Resource       string           `json:"resource"`
	Name           string           `json:"name"`
	Kind           string           `json:"kind"`
	Team           string           `json:"team,omitempty"`
	Owner          string           `json:"owner,omitempty"`
	SharedResource string           `json:"shared_resource"`
	DependencyKind string           `json:"dependency_kind"`
	Confidence     model.Confidence `json:"confidence"`
	Sources        []string         `json:"sources"`
	Headroom       []model.Limit    `json:"headroom"`
}

// Shared is a resource consumed by two or more teams that the change can reach.
type Shared struct {
	Resource string        `json:"resource"`
	Kind     string        `json:"kind"`
	Teams    []string      `json:"teams"`
	Limits   []model.Limit `json:"limits"`
	Tight    bool          `json:"tight"`
}

// Coverage describes what the check could and could not see.
type Coverage struct {
	SourcesLoaded         []string `json:"sources_loaded"`
	EdgeSources           []string `json:"edge_sources"`
	ResourcesWithoutOwner []string `json:"resources_without_owner"`
	LowConfidenceEdges    []string `json:"low_confidence_edges"`
}

// Tight reports whether a limit is at, past, or within TightFraction of its threshold.
func Tight(limit model.Limit, fraction float64) bool {
	if fraction <= 0 {
		fraction = DefaultTightFraction
	}
	if limit.Headroom <= 0 {
		return true
	}
	denom := limit.Threshold
	if denom < 0 {
		denom = -denom
	}
	if denom == 0 {
		return false
	}
	return limit.Headroom/denom < fraction
}

// Run builds an exposure result. limits may be empty when metrics were not loaded.
// sources are the loader names the caller actually used.
func Run(g *graph.Graph, changes []model.Change, limits []model.Limit, sources []string, opt Options) (Result, error) {
	if g == nil {
		return Result{}, errors.New("graph is nil")
	}
	changes = append([]model.Change(nil), changes...)
	for _, change := range changes {
		if err := change.Validate(); err != nil {
			return Result{}, fmt.Errorf("change: %w", err)
		}
	}
	model.SortChanges(changes)

	limitsBy := map[string][]model.Limit{}
	for _, limit := range limits {
		if err := limit.Validate(); err != nil {
			return Result{}, fmt.Errorf("limit: %w", err)
		}
		limitsBy[limit.Resource] = append(limitsBy[limit.Resource], limit)
	}
	for id := range limitsBy {
		model.SortLimits(limitsBy[id])
	}

	sharedByID := map[string]model.Resource{}
	for _, res := range g.SharedResources() {
		sharedByID[res.ID] = res
	}
	changed := map[string]model.Change{}
	for _, change := range changes {
		changed[change.Resource] = change
	}
	touched := blast(g, changed)

	var exposed []model.Resource
	for id, res := range sharedByID {
		if _, ok := touched[id]; ok {
			exposed = append(exposed, res)
		}
	}
	model.SortResources(exposed)

	fraction := opt.fraction()
	var (
		affected []Affected
		shared   []Shared
		reasons  []string
		high     bool
		warn     bool
	)
	for _, change := range changes {
		if len(change.ChangedAttrs) == 0 {
			reasons = append(reasons, fmt.Sprintf("%s %s.", change.Action, change.Resource))
			continue
		}
		reasons = append(reasons, fmt.Sprintf("%s %s changes %s.", change.Action, change.Resource, strings.Join(change.ChangedAttrs, ", ")))
	}

	for _, res := range exposed {
		teams := graph.TeamsOf(g, res.ID)
		lims := copyLimits(limitsBy[res.ID])
		item := Shared{
			Resource: res.ID,
			Kind:     res.Kind,
			Teams:    teams,
			Limits:   lims,
		}
		if teams == nil {
			item.Teams = []string{}
		}
		reasons = append(reasons, fmt.Sprintf("%s is shared by %s.", res.ID, joinTeams(teams)))
		if change, ok := changed[res.ID]; ok && (change.Action == model.ActionDelete || change.Action == model.ActionReplace) {
			item.Tight = true
			high = true
			reasons = append(reasons, fmt.Sprintf("Shared resource %s is being %sd.", res.ID, change.Action))
		}
		resourceTight := false
		for _, limit := range lims {
			if !Tight(limit, fraction) {
				continue
			}
			resourceTight = true
			high = true
			reasons = append(reasons, fmt.Sprintf(
				"%s on %s has headroom %s (current %s, threshold %s).",
				limit.Metric, res.ID, formatNum(limit.Headroom), formatNum(limit.Current), formatNum(limit.Threshold),
			))
		}
		if resourceTight {
			item.Tight = true
		}
		if !item.Tight {
			warn = true
			if len(lims) == 0 {
				reasons = append(reasons, fmt.Sprintf("Headroom for shared resource %s is unknown.", res.ID))
			} else {
				reasons = append(reasons, fmt.Sprintf("Shared resource %s is inside its limits.", res.ID))
			}
		}
		shared = append(shared, item)

		for _, dep := range g.Incoming(res.ID) {
			if _, isChange := changed[dep.From]; isChange {
				continue
			}
			consumer, ok := g.Resource(dep.From)
			if !ok {
				continue
			}
			affected = append(affected, Affected{
				Resource:       consumer.ID,
				Name:           consumer.Name,
				Kind:           consumer.Kind,
				Team:           consumer.Team,
				Owner:          consumer.Owner,
				SharedResource: res.ID,
				DependencyKind: dep.Kind,
				Confidence:     dep.Confidence,
				Sources:        copyStrings(dep.Sources),
				Headroom:       copyLimits(lims),
			})
		}
	}
	sort.Slice(affected, func(i, j int) bool {
		if affected[i].Name != affected[j].Name {
			return affected[i].Name < affected[j].Name
		}
		if affected[i].DependencyKind != affected[j].DependencyKind {
			return affected[i].DependencyKind < affected[j].DependencyKind
		}
		return affected[i].SharedResource < affected[j].SharedResource
	})

	if len(exposed) == 0 {
		if others := otherTeams(g, changed); len(others) > 0 {
			warn = true
			reasons = append(reasons, fmt.Sprintf("The change reaches %s.", strings.Join(others, ", ")))
		}
	}

	verdict := VerdictInfo
	switch {
	case high:
		verdict = VerdictHigh
		reasons = append(reasons, "Verdict is high-exposure because a shared resource is at or past a limit, or is being deleted or replaced.")
	case warn:
		verdict = VerdictWarn
		reasons = append(reasons, "Verdict is warn because the change reaches a resource other teams depend on and headroom is not past a limit.")
	default:
		reasons = append(reasons, "Verdict is info because the change does not reach a resource another team depends on.")
	}

	if affected == nil {
		affected = []Affected{}
	}
	if shared == nil {
		shared = []Shared{}
	}
	if reasons == nil {
		reasons = []string{}
	}
	result := Result{
		SchemaVersion: SchemaVersion,
		Verdict:       verdict,
		Reasons:       reasons,
		Changes:       changes,
		Affected:      affected,
		Shared:        shared,
		Teams:         teamsOf(affected),
		Owners:        ownersOf(affected),
		Coverage:      coverage(g, sources),
	}
	if result.Changes == nil {
		result.Changes = []model.Change{}
	}
	return result, nil
}

func blast(g *graph.Graph, changed map[string]model.Change) map[string]struct{} {
	touched := map[string]struct{}{}
	mark := func(id string) {
		touched[id] = struct{}{}
		for _, up := range g.Upstream(id) {
			touched[up] = struct{}{}
		}
	}
	downstream := map[string]struct{}{}
	for id := range changed {
		mark(id)
		for _, down := range g.Downstream(id) {
			downstream[down] = struct{}{}
		}
	}
	for id := range downstream {
		mark(id)
	}
	return touched
}

func otherTeams(g *graph.Graph, changed map[string]model.Change) []string {
	seen := map[string]string{}
	consider := func(id string) {
		if _, ok := changed[id]; ok {
			return
		}
		res, ok := g.Resource(id)
		if !ok {
			return
		}
		team := model.DisplayTeam(res.Team, res.Owner)
		if team == "" {
			return
		}
		key := model.TeamKey(team)
		if _, ok := seen[key]; !ok {
			seen[key] = team
		}
	}
	for id := range changed {
		for _, up := range g.Upstream(id) {
			for _, dep := range g.Incoming(up) {
				consider(dep.From)
			}
		}
		for _, down := range g.Downstream(id) {
			consider(down)
		}
	}
	out := make([]string, 0, len(seen))
	for _, team := range seen {
		out = append(out, team)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func coverage(g *graph.Graph, sources []string) Coverage {
	cov := Coverage{
		SourcesLoaded:         model.SortStrings(sources),
		EdgeSources:           []string{},
		ResourcesWithoutOwner: []string{},
		LowConfidenceEdges:    []string{},
	}
	if cov.SourcesLoaded == nil {
		cov.SourcesLoaded = []string{}
	}
	edgeSources := map[string]struct{}{}
	for _, dep := range g.Dependencies() {
		edgeSources[dep.Source] = struct{}{}
		for _, source := range dep.Sources {
			edgeSources[source] = struct{}{}
		}
		if dep.Confidence == model.ConfidenceLow {
			cov.LowConfidenceEdges = append(cov.LowConfidenceEdges, fmt.Sprintf("%s -> %s %s (%s)", dep.From, dep.To, dep.Kind, dep.Source))
		}
	}
	for source := range edgeSources {
		cov.EdgeSources = append(cov.EdgeSources, source)
	}
	sort.Strings(cov.EdgeSources)
	sort.Strings(cov.LowConfidenceEdges)
	for _, res := range g.Resources() {
		if res.Team == "" && res.Owner == "" {
			cov.ResourcesWithoutOwner = append(cov.ResourcesWithoutOwner, res.ID)
		}
	}
	return cov
}

func teamsOf(affected []Affected) []string {
	seen := map[string]string{}
	for _, item := range affected {
		team := model.DisplayTeam(item.Team, item.Owner)
		if team == "" {
			continue
		}
		key := model.TeamKey(team)
		if _, ok := seen[key]; !ok {
			seen[key] = team
		}
	}
	out := make([]string, 0, len(seen))
	for _, team := range seen {
		out = append(out, team)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	if out == nil {
		return []string{}
	}
	return out
}

func ownersOf(affected []Affected) []string {
	var owners []string
	for _, item := range affected {
		if item.Owner != "" {
			owners = append(owners, item.Owner)
		}
	}
	owners = model.SortStrings(owners)
	if owners == nil {
		return []string{}
	}
	return owners
}

func joinTeams(teams []string) string {
	if len(teams) == 0 {
		return "no named team"
	}
	return strings.Join(teams, ", ")
}

func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func copyLimits(in []model.Limit) []model.Limit {
	out := make([]model.Limit, len(in))
	copy(out, in)
	return out
}

func copyStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

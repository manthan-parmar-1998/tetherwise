package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// SchemaVersion is the snapshot document version written and accepted.
const SchemaVersion = "v0.1"

// Graph is an in-memory dependency graph.
type Graph struct {
	resources map[string]model.Resource
	deps      []model.Dependency
	index     map[string]int
}

// New returns an empty graph.
func New() *Graph {
	return &Graph{
		resources: map[string]model.Resource{},
		index:     map[string]int{},
	}
}

// Merge adds resources and dependencies.
//
// Resources with the same ID keep the first non-empty owner, team, and
// attribute value. Dependencies with the same from, to, and kind keep the
// highest confidence. The winning observation's source stays Source, and
// Sources lists every observation.
//
// A dependency endpoint that is not already a resource is inserted as an
// identity stub so later sources can fill it in.
func (g *Graph) Merge(resources []model.Resource, deps []model.Dependency) error {
	for _, res := range resources {
		if err := res.Validate(); err != nil {
			return fmt.Errorf("merge resource: %w", err)
		}
		g.addResource(res)
	}
	for _, dep := range deps {
		if err := g.addDep(dep); err != nil {
			return err
		}
	}
	return nil
}

func (g *Graph) addResource(res model.Resource) {
	prev, ok := g.resources[res.ID]
	if !ok {
		if res.Attrs == nil {
			res.Attrs = map[string]string{}
		}
		g.resources[res.ID] = res
		return
	}
	if prev.Team == "" {
		prev.Team = res.Team
	}
	if prev.Owner == "" {
		prev.Owner = res.Owner
	}
	if prev.Attrs == nil {
		prev.Attrs = map[string]string{}
	}
	for k, v := range res.Attrs {
		if _, exists := prev.Attrs[k]; !exists {
			prev.Attrs[k] = v
		}
	}
	g.resources[res.ID] = prev
}

func (g *Graph) addDep(dep model.Dependency) error {
	if len(dep.Sources) == 0 && dep.Source != "" {
		dep.Sources = []string{dep.Source}
	}
	dep.Sources = model.SortStrings(dep.Sources)
	if err := dep.Validate(); err != nil {
		return fmt.Errorf("merge dependency: %w", err)
	}
	if err := g.ensure(dep.From); err != nil {
		return err
	}
	if err := g.ensure(dep.To); err != nil {
		return err
	}
	key := dep.From + "\x00" + dep.To + "\x00" + dep.Kind
	if i, ok := g.index[key]; ok {
		g.deps[i] = mergeDep(g.deps[i], dep)
		return nil
	}
	g.index[key] = len(g.deps)
	g.deps = append(g.deps, dep)
	return nil
}

func (g *Graph) ensure(id string) error {
	if _, ok := g.resources[id]; ok {
		return nil
	}
	res, err := model.ResourceFromID(id)
	if err != nil {
		return fmt.Errorf("dependency endpoint: %w", err)
	}
	res.Attrs = map[string]string{}
	g.resources[id] = res
	return nil
}

func mergeDep(a, b model.Dependency) model.Dependency {
	keep, add := a, b
	if better(b, a) {
		keep, add = b, a
	}
	keep.Sources = model.SortStrings(append(append([]string{}, keep.Sources...), add.Sources...))
	if !contains(keep.Sources, keep.Source) {
		keep.Sources = model.SortStrings(append(keep.Sources, keep.Source))
	}
	return keep
}

func better(candidate, current model.Dependency) bool {
	if candidate.Confidence.Rank() != current.Confidence.Rank() {
		return candidate.Confidence.Rank() > current.Confidence.Rank()
	}
	return sourceRank(candidate.Source) < sourceRank(current.Source)
}

func sourceRank(source string) int {
	switch source {
	case model.SourceEdgesFile:
		return 0
	case model.SourceTerraform:
		return 1
	case model.SourceFluentBit:
		return 2
	case model.SourceVector:
		return 3
	case model.SourceK8sEnv:
		return 4
	default:
		return 100
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// Resource returns one resource by ID.
func (g *Graph) Resource(id string) (model.Resource, bool) {
	res, ok := g.resources[id]
	return res, ok
}

// Resources returns resources ordered by ID.
func (g *Graph) Resources() []model.Resource {
	out := make([]model.Resource, 0, len(g.resources))
	for _, res := range g.resources {
		out = append(out, res)
	}
	model.SortResources(out)
	return out
}

// Dependencies returns edges ordered by from, to, and kind.
func (g *Graph) Dependencies() []model.Dependency {
	out := append([]model.Dependency(nil), g.deps...)
	model.SortDependencies(out)
	return out
}

// Outgoing returns edges from id, ordered stably.
func (g *Graph) Outgoing(id string) []model.Dependency {
	var out []model.Dependency
	for _, dep := range g.deps {
		if dep.From == id {
			out = append(out, dep)
		}
	}
	model.SortDependencies(out)
	return out
}

// Incoming returns edges to id, ordered stably.
func (g *Graph) Incoming(id string) []model.Dependency {
	var out []model.Dependency
	for _, dep := range g.deps {
		if dep.To == id {
			out = append(out, dep)
		}
	}
	model.SortDependencies(out)
	return out
}

// Upstream returns resources id depends on, transitively, excluding id.
func (g *Graph) Upstream(id string) []string {
	return g.walk(id, func(cur string) []string {
		var next []string
		for _, dep := range g.deps {
			if dep.From == cur {
				next = append(next, dep.To)
			}
		}
		return next
	})
}

// Downstream returns resources that depend on id, transitively, excluding id.
func (g *Graph) Downstream(id string) []string {
	return g.walk(id, func(cur string) []string {
		var next []string
		for _, dep := range g.deps {
			if dep.To == cur {
				next = append(next, dep.From)
			}
		}
		return next
	})
}

func (g *Graph) walk(start string, next func(string) []string) []string {
	seen := map[string]struct{}{start: {}}
	queue := []string{start}
	var out []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, id := range next(cur) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
			queue = append(queue, id)
		}
	}
	sort.Strings(out)
	return out
}

// SharedResources returns resources whose direct consumers belong to two
// or more teams. A consumer with neither team nor owner does not count.
func (g *Graph) SharedResources() []model.Resource {
	var out []model.Resource
	for _, res := range g.Resources() {
		teams := map[string]struct{}{}
		for _, dep := range g.Incoming(res.ID) {
			consumer, ok := g.resources[dep.From]
			if !ok {
				continue
			}
			team := model.DisplayTeam(consumer.Team, consumer.Owner)
			if team == "" {
				continue
			}
			teams[model.TeamKey(team)] = struct{}{}
		}
		if len(teams) >= 2 {
			out = append(out, res)
		}
	}
	return out
}

// Snapshot is the versioned on-disk graph.
type Snapshot struct {
	SchemaVersion string             `json:"schema_version"`
	Resources     []model.Resource   `json:"resources"`
	Dependencies  []model.Dependency `json:"dependencies"`
}

// Snapshot returns a stable document.
func (g *Graph) Snapshot() Snapshot {
	resources := g.Resources()
	deps := g.Dependencies()
	if resources == nil {
		resources = []model.Resource{}
	}
	if deps == nil {
		deps = []model.Dependency{}
	}
	for i := range resources {
		if resources[i].Attrs == nil {
			resources[i].Attrs = map[string]string{}
		}
	}
	return Snapshot{
		SchemaVersion: SchemaVersion,
		Resources:     resources,
		Dependencies:  deps,
	}
}

// WriteJSON writes a snapshot.
func (g *Graph) WriteJSON(w io.Writer) error {
	buf, err := model.Marshal(g.Snapshot())
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	if _, err := w.Write(buf); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// ReadJSON reads a snapshot into a new graph.
func ReadJSON(r io.Reader) (*Graph, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(buf, &snap); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	if snap.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("snapshot schema_version %q, want %s", snap.SchemaVersion, SchemaVersion)
	}
	g := New()
	if err := g.Merge(snap.Resources, snap.Dependencies); err != nil {
		return nil, err
	}
	return g, nil
}

// TeamsOf lists the distinct consumer teams of a resource, display form, sorted.
func TeamsOf(g *Graph, id string) []string {
	seen := map[string]string{}
	for _, dep := range g.Incoming(id) {
		consumer, ok := g.Resource(dep.From)
		if !ok {
			continue
		}
		team := model.DisplayTeam(consumer.Team, consumer.Owner)
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
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

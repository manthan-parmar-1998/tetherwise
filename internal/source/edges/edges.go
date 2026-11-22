package edges

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

const (
	// APIVersion is the only schema version read by v0.1.
	APIVersion = "tetherwise.io/v1"
	// Kind is the document kind.
	Kind = "Edges"
)

// File is a tetherwise.edges.yaml document.
type File struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
	Edges      []Edge `json:"edges" yaml:"edges"`
}

// Edge is one declared dependency. Confidence and source are not fields:
// the file always means a high-confidence edges-file observation.
type Edge struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
	Kind string `json:"kind" yaml:"kind"`
}

// ReadFile parses an edges file into dependencies.
func ReadFile(path string) ([]model.Dependency, error) {
	buf, err := os.ReadFile(path) //nolint:gosec // G304: operator-selected edges file
	if err != nil {
		return nil, fmt.Errorf("open edges file: %w", err)
	}
	deps, err := Parse(buf)
	if err != nil {
		return nil, fmt.Errorf("parse edges file %s: %w", path, err)
	}
	return deps, nil
}

// Parse decodes an edges document.
func Parse(buf []byte) ([]model.Dependency, error) {
	var file File
	if err := yaml.Unmarshal(buf, &file); err != nil {
		return nil, fmt.Errorf("decode edges yaml: %w", err)
	}
	if file.APIVersion != APIVersion {
		return nil, fmt.Errorf("edges apiVersion %q, want %s", file.APIVersion, APIVersion)
	}
	if file.Kind != Kind {
		return nil, fmt.Errorf("edges kind %q, want %s", file.Kind, Kind)
	}
	deps := make([]model.Dependency, 0, len(file.Edges))
	for i, edge := range file.Edges {
		dep := model.Dependency{
			From: edge.From, To: edge.To, Kind: edge.Kind,
			Source:     model.SourceEdgesFile,
			Sources:    []string{model.SourceEdgesFile},
			Confidence: model.ConfidenceHigh,
		}
		if err := dep.Validate(); err != nil {
			return nil, fmt.Errorf("edges[%d]: %w", i, err)
		}
		deps = append(deps, dep)
	}
	model.SortDependencies(deps)
	return deps, nil
}

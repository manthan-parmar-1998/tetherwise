package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

const fixtureSchema = "v0.1"

// Fixture replays recorded samples and computes headroom locally.
type Fixture struct {
	samples    []fixtureSample
	thresholds Thresholds
	byID       map[string]model.Resource
}

type fixtureFile struct {
	SchemaVersion string          `json:"schema_version"`
	Samples       []fixtureSample `json:"samples"`
}

type fixtureSample struct {
	Resource string  `json:"resource"`
	Metric   string  `json:"metric"`
	Current  float64 `json:"current"`
	Unit     string  `json:"unit,omitempty"`
}

// LoadFixture reads a recorded metrics file.
func LoadFixture(path string, thresholds Thresholds) (*Fixture, error) {
	buf, err := os.ReadFile(path) //nolint:gosec // G304: operator-selected metrics fixture
	if err != nil {
		return nil, fmt.Errorf("open metrics fixture: %w", err)
	}
	f, err := ParseFixture(buf, thresholds)
	if err != nil {
		return nil, fmt.Errorf("parse metrics fixture %s: %w", path, err)
	}
	return f, nil
}

// ParseFixture decodes a fixture document.
func ParseFixture(buf []byte, thresholds Thresholds) (*Fixture, error) {
	var file fixtureFile
	if err := json.Unmarshal(buf, &file); err != nil {
		return nil, fmt.Errorf("decode metrics fixture: %w", err)
	}
	if file.SchemaVersion != fixtureSchema {
		return nil, fmt.Errorf("metrics fixture schema_version %q, want %s", file.SchemaVersion, fixtureSchema)
	}
	return &Fixture{
		samples:    file.Samples,
		thresholds: thresholds.WithDefaults(),
		byID:       map[string]model.Resource{},
	}, nil
}

// Headroom returns limits for samples whose resource was requested.
func (f *Fixture) Headroom(_ context.Context, resources []model.Resource) ([]model.Limit, error) {
	known := map[string]model.Resource{}
	for _, res := range resources {
		known[res.ID] = res
	}
	var limits []model.Limit
	for _, sample := range f.samples {
		res, ok := known[sample.Resource]
		if !ok {
			continue
		}
		spec, ok := specByName(res.Kind, sample.Metric)
		if !ok {
			return nil, fmt.Errorf("fixture metric %s is not tracked for %s", sample.Metric, res.Kind)
		}
		unit := spec.unit
		if sample.Unit != "" {
			unit = sample.Unit
		}
		limit, err := model.NewLimit(res.ID, spec.name, spec.threshold(f.thresholds), sample.Current, unit, spec.direction)
		if err != nil {
			return nil, err
		}
		limits = append(limits, limit)
	}
	model.SortLimits(limits)
	if limits == nil {
		limits = []model.Limit{}
	}
	return limits, nil
}

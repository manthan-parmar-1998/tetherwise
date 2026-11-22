package model

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceIDTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		provider  string
		kind      string
		resName   string
		wantID    string
		wantError bool
	}{
		{
			name:     "opensearch",
			provider: "aws", kind: "opensearch:domain", resName: "search-lab",
			wantID: "aws:opensearch:domain/search-lab",
		},
		{
			name:     "deployment",
			provider: "k8s", kind: "deployment", resName: "atlas/atlas-indexer",
			wantID: "k8s:deployment/atlas/atlas-indexer",
		},
		{
			name:     "log group path",
			provider: "aws", kind: "logs:log-group", resName: "/eks/lab",
			wantID: "aws:logs:log-group//eks/lab",
		},
		{
			name:     "empty name",
			provider: "aws", kind: "rds:instance", resName: "",
			wantError: true,
		},
		{
			name:     "upper provider",
			provider: "AWS", kind: "rds:instance", resName: "db",
			wantError: true,
		},
		{
			name:     "bad kind",
			provider: "aws", kind: "RDS", resName: "db",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResourceID(tt.provider, tt.kind, tt.resName)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantID {
				t.Fatalf("id = %s, want %s", got, tt.wantID)
			}
			provider, kind, name, err := ParseID(got)
			if err != nil {
				t.Fatal(err)
			}
			if provider != tt.provider || kind != tt.kind || name != tt.resName {
				t.Fatalf("parsed %s %s %s", provider, kind, name)
			}
		})
	}
}

func TestParseIDRejects(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"", "noslash", "aws:", "/name", "AWS:rds:instance/db", "aws:Kind/name"} {
		if _, _, _, err := ParseID(id); err == nil {
			t.Fatalf("ParseID(%q) succeeded", id)
		}
	}
}

func TestValidationTable(t *testing.T) {
	t.Parallel()
	validRes := Resource{
		ID: "aws:opensearch:domain/search-lab", Provider: "aws",
		Kind: "opensearch:domain", Name: "search-lab",
		Team: "Atlas", Owner: "atlas-oncall",
		Attrs: map[string]string{"endpoint": "search-lab.example.local"},
	}
	if err := validRes.Validate(); err != nil {
		t.Fatal(err)
	}

	badProvider := validRes
	badProvider.Provider = "k8s"
	if err := badProvider.Validate(); err == nil {
		t.Fatal("mismatched provider")
	}

	badDep := Dependency{
		From: validRes.ID, To: validRes.ID, Kind: KindConnectsTo,
		Source: SourceTerraform, Confidence: ConfidenceHigh,
	}
	if err := badDep.Validate(); err == nil {
		t.Fatal("self edge")
	}

	tests := []struct {
		name string
		dep  Dependency
		ok   bool
	}{
		{
			name: "high fluentbit",
			dep: Dependency{
				From: "k8s:deployment/atlas/atlas-indexer", To: validRes.ID,
				Kind: KindWritesLogsTo, Source: SourceFluentBit,
				Sources: []string{SourceFluentBit}, Confidence: ConfidenceHigh,
			},
			ok: true,
		},
		{
			name: "sources missing primary",
			dep: Dependency{
				From: "k8s:deployment/atlas/atlas-indexer", To: validRes.ID,
				Kind: KindReadsFrom, Source: SourceEdgesFile,
				Sources: []string{SourceTerraform}, Confidence: ConfidenceHigh,
			},
		},
		{
			name: "bad kind",
			dep: Dependency{
				From: "k8s:deployment/atlas/atlas-indexer", To: validRes.ID,
				Kind: "calls", Source: SourceTerraform, Confidence: ConfidenceLow,
			},
		},
		{
			name: "bad confidence",
			dep: Dependency{
				From: "k8s:deployment/atlas/atlas-indexer", To: validRes.ID,
				Kind: KindConnectsTo, Source: SourceK8sEnv, Confidence: "sure",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.dep.Validate()
			if tt.ok && err != nil {
				t.Fatal(err)
			}
			if !tt.ok && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestLimitHeadroom(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		direction Direction
		threshold float64
		current   float64
		want      float64
	}{
		{name: "pressure over", direction: DirectionLowerBetter, threshold: 85, current: 92, want: -7},
		{name: "cpu inside", direction: DirectionLowerBetter, threshold: 80, current: 40, want: 40},
		{name: "free storage short", direction: DirectionHigherBetter, threshold: 20480, current: 6000, want: -14480},
		{name: "free storage ok", direction: DirectionHigherBetter, threshold: 10, current: 25, want: 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ComputeHeadroom(tt.direction, tt.threshold, tt.current)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("headroom = %v, want %v", got, tt.want)
			}
			limit, err := NewLimit("aws:opensearch:domain/search-lab", "JVMMemoryPressure", tt.threshold, tt.current, "Percent", tt.direction)
			if err != nil {
				t.Fatal(err)
			}
			if limit.Headroom != tt.want {
				t.Fatalf("limit headroom = %v", limit.Headroom)
			}
		})
	}

	if _, err := ComputeHeadroom("sideways", 1, 1); err == nil {
		t.Fatal("bad direction")
	}
	bad, err := NewLimit("aws:opensearch:domain/search-lab", "CPUUtilization", 80, 10, "Percent", DirectionLowerBetter)
	if err != nil {
		t.Fatal(err)
	}
	bad.Headroom = 0
	if err := bad.Validate(); err == nil {
		t.Fatal("headroom mismatch")
	}
}

func TestChangeValidation(t *testing.T) {
	t.Parallel()
	ok := Change{Resource: "aws:logs:subscription-filter/eks-logs", Action: ActionUpdate, ChangedAttrs: []string{"filter_pattern"}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	dup := ok
	dup.ChangedAttrs = []string{"filter_pattern", "filter_pattern"}
	if err := dup.Validate(); err == nil {
		t.Fatal("duplicate attr")
	}
	badAction := ok
	badAction.Action = "noop"
	if err := badAction.Validate(); err == nil {
		t.Fatal("bad action")
	}
}

func TestConfidenceRank(t *testing.T) {
	t.Parallel()
	if ConfidenceHigh.Rank() <= ConfidenceMedium.Rank() || ConfidenceMedium.Rank() <= ConfidenceLow.Rank() {
		t.Fatal("rank order")
	}
	if Confidence("nope").Valid() {
		t.Fatal("unknown confidence")
	}
}

func TestSortStringsUnique(t *testing.T) {
	t.Parallel()
	got := SortStrings([]string{"b", "a", "b", "a"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("%v", got)
	}
	if SortStrings(nil) != nil {
		t.Fatal("nil")
	}
}

func TestGoldenJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		val  any
	}{
		{
			name: "resource.json",
			val: Resource{
				ID: "aws:opensearch:domain/search-lab", Provider: "aws",
				Kind: "opensearch:domain", Name: "search-lab",
				Owner: "platform-oncall", Team: "Platform",
				Attrs: map[string]string{
					"endpoint":    "search-lab.us-east-1.es.amazonaws.com",
					"domain_name": "search-lab",
				},
			},
		},
		{
			name: "dependency.json",
			val: Dependency{
				From: "k8s:deployment/atlas/atlas-indexer", To: "aws:opensearch:domain/search-lab",
				Kind: KindWritesLogsTo, Source: SourceFluentBit,
				Sources: []string{SourceFluentBit, SourceK8sEnv}, Confidence: ConfidenceHigh,
			},
		},
		{
			name: "limit.json",
			val: Limit{
				Resource: "aws:opensearch:domain/search-lab", Metric: "JVMMemoryPressure",
				Threshold: 85, Current: 92, Headroom: -7, Unit: "Percent",
				Direction: DirectionLowerBetter,
			},
		},
		{
			name: "change.json",
			val: Change{
				Resource: "aws:logs:subscription-filter/eks-logs", Action: ActionUpdate,
				ChangedAttrs: []string{"filter_pattern"},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Marshal(tt.val)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tt.name)
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestValidationErrorIs(t *testing.T) {
	t.Parallel()
	err := invalid("resource", "id", "bad")
	var target *ValidationError
	if !errors.As(err, &target) {
		t.Fatal("errors.As")
	}
	if target.Error() == "" {
		t.Fatal("empty error")
	}
}

func TestResourceAttrKey(t *testing.T) {
	t.Parallel()
	r := Resource{
		ID: "aws:rds:instance/payments", Provider: "aws", Kind: "rds:instance", Name: "payments",
		Attrs: map[string]string{" ": "x"},
	}
	if err := r.Validate(); err == nil {
		t.Fatal("blank attr key")
	}
}

func TestDisplayTeam(t *testing.T) {
	t.Parallel()
	if DisplayTeam(" Atlas ", "other") != "Atlas" {
		t.Fatal("team")
	}
	if DisplayTeam("  ", " keel ") != "keel" {
		t.Fatal("owner")
	}
	if TeamKey(" Atlas ") != "atlas" {
		t.Fatal("key")
	}
}

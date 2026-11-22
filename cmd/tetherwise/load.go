package main

import (
	"context"
	"fmt"
	"os"

	"github.com/manthan-parmar-1998/tetherwise/internal/graph"
	"github.com/manthan-parmar-1998/tetherwise/internal/model"
	"github.com/manthan-parmar-1998/tetherwise/internal/source/edges"
	k8ssource "github.com/manthan-parmar-1998/tetherwise/internal/source/kubernetes"
	"github.com/manthan-parmar-1998/tetherwise/internal/source/terraform"
)

func (a *app) loadGraph(ctx context.Context, tfStates, tfPlans, k8sPaths []string, edgesPath, snapshot string, live bool) (*graph.Graph, []model.Change, []string, error) {
	g := graph.New()
	var sources []string
	var changes []model.Change
	if snapshot != "" {
		f, err := os.Open(snapshot) //nolint:gosec // G304: operator-selected snapshot
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open snapshot: %w", err)
		}
		loaded, err := graph.ReadJSON(f)
		_ = f.Close()
		if err != nil {
			return nil, nil, nil, err
		}
		g = loaded
		sources = append(sources, "snapshot")
	}
	for _, path := range tfStates {
		resources, deps, err := terraform.ParseStateFile(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := g.Merge(resources, deps); err != nil {
			return nil, nil, nil, err
		}
		sources = append(sources, "terraform")
	}
	for _, path := range tfPlans {
		resources, deps, planChanges, err := loadPlan(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := g.Merge(resources, deps); err != nil {
			return nil, nil, nil, err
		}
		changes = append(changes, planChanges...)
		sources = append(sources, "terraform")
	}
	if edgesPath != "" {
		deps, err := edges.ReadFile(edgesPath)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := g.Merge(nil, deps); err != nil {
			return nil, nil, nil, err
		}
		sources = append(sources, "edges-file")
	}
	if len(k8sPaths) > 0 {
		resources, deps, err := k8ssource.ReadFiles(k8sPaths, g.Resources())
		if err != nil {
			return nil, nil, nil, err
		}
		if err := g.Merge(resources, deps); err != nil {
			return nil, nil, nil, err
		}
		sources = append(sources, "kubernetes")
	}
	if live {
		if a.kube == nil {
			return nil, nil, nil, fmt.Errorf("kubernetes client is not configured")
		}
		resources, deps, err := k8ssource.ReadCluster(contextOrBackground(ctx), a.kube, g.Resources(), a.audit)
		if err != nil {
			return nil, nil, nil, err
		}
		if err := g.Merge(resources, deps); err != nil {
			return nil, nil, nil, err
		}
		sources = append(sources, "kubernetes")
	}
	return g, changes, model.SortStrings(sources), nil
}

func loadPlan(path string) ([]model.Resource, []model.Dependency, []model.Change, error) {
	return terraform.ParsePlanFile(path)
}

package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func domain() model.Resource {
	return model.Resource{
		ID: "aws:opensearch:domain/search-lab", Provider: "aws", Kind: "opensearch:domain", Name: "search-lab",
		Attrs: map[string]string{"endpoint": "search-lab.us-east-1.es.amazonaws.com"},
	}
}

func TestReadFiles(t *testing.T) {
	t.Parallel()
	resources, deps, err := ReadFiles([]string{"testdata"}, []model.Resource{domain()})
	if err != nil {
		t.Fatal(err)
	}
	deploy, ok := find(resources, "k8s:deployment/atlas/atlas-indexer")
	if !ok {
		t.Fatalf("deployment missing: %v", resources)
	}
	if deploy.Team != "Atlas" || deploy.Owner != "atlas-oncall" {
		t.Fatalf("owner = %s %s", deploy.Team, deploy.Owner)
	}
	if !has(deps, deploy.ID, domain().ID, model.KindConnectsTo, model.SourceK8sEnv, model.ConfidenceHigh) {
		t.Fatalf("env edge missing: %#v", deps)
	}
	if !has(deps, deploy.ID, domain().ID, model.KindReadsFrom, model.SourceK8sEnv, model.ConfidenceHigh) {
		t.Fatalf("query edge missing: %#v", deps)
	}
	if !has(deps, deploy.ID, domain().ID, model.KindWritesLogsTo, model.SourceFluentBit, model.ConfidenceHigh) {
		t.Fatalf("fluent bit edge missing: %#v", deps)
	}
	if !has(deps, deploy.ID, domain().ID, model.KindWritesLogsTo, model.SourceVector, model.ConfidenceHigh) {
		t.Fatalf("vector edge missing: %#v", deps)
	}
}

func TestReadFilesNameMatch(t *testing.T) {
	t.Parallel()
	known := domain()
	delete(known.Attrs, "endpoint")
	_, deps, err := ReadFiles([]string{"testdata/workloads.yaml"}, []model.Resource{known})
	if err != nil {
		t.Fatal(err)
	}
	if !has(deps, "k8s:deployment/atlas/atlas-indexer", known.ID, model.KindConnectsTo, model.SourceK8sEnv, model.ConfidenceMedium) {
		t.Fatalf("medium match missing: %#v", deps)
	}
}

func TestReadCluster(t *testing.T) {
	t.Parallel()
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keel-query", Namespace: "keel",
			Labels: map[string]string{"team": "Keel", "owner": "keel-oncall"},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "query",
						Env: []corev1.EnvVar{{
							Name: "LOG_TARGET", Value: "search-lab.us-east-1.es.amazonaws.com",
						}},
					}},
				},
			},
		},
	}
	client := fake.NewClientset(deploy)
	rec := &memAudit{}
	resources, deps, err := ReadCluster(context.Background(), client, []model.Resource{domain()}, rec)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := find(resources, "k8s:deployment/keel/keel-query"); !ok {
		t.Fatalf("live deployment missing: %v", resources)
	}
	if !has(deps, "k8s:deployment/keel/keel-query", domain().ID, model.KindWritesLogsTo, model.SourceK8sEnv, model.ConfidenceHigh) {
		t.Fatalf("live edge missing: %#v", deps)
	}
	if len(rec.events) != 5 {
		t.Fatalf("audit events = %d", len(rec.events))
	}
}

func TestFluentAndVectorParsers(t *testing.T) {
	t.Parallel()
	if got := fluentBitHosts("not a config"); got != nil {
		t.Fatal(got)
	}
	if got := vectorEndpoints("just text"); got != nil {
		t.Fatal(got)
	}
	hosts := fluentBitHosts("[OUTPUT]\nName stdout\nHost ignored.example\n")
	if len(hosts) != 0 {
		t.Fatalf("non-search plugin: %v", hosts)
	}
}

func TestMatchKnown(t *testing.T) {
	t.Parallel()
	if matchKnown("", nil) != nil {
		t.Fatal("empty")
	}
	short := model.Resource{ID: "aws:rds:instance/db", Kind: "rds:instance", Name: "db", Attrs: map[string]string{"endpoint": "short"}}
	if _, ok := confidenceFor("short", short); ok {
		t.Fatal("short endpoint should not match")
	}
}

type memAudit struct {
	events []string
}

func (m *memAudit) Record(service, operation, target string) error {
	m.events = append(m.events, service+" "+operation+" "+target)
	return nil
}

func find(resources []model.Resource, id string) (model.Resource, bool) {
	for _, res := range resources {
		if res.ID == id {
			return res, true
		}
	}
	return model.Resource{}, false
}

func has(deps []model.Dependency, from, to, kind, source string, conf model.Confidence) bool {
	for _, dep := range deps {
		if dep.From == from && dep.To == to && dep.Kind == kind && dep.Source == source && dep.Confidence == conf {
			return true
		}
	}
	return false
}

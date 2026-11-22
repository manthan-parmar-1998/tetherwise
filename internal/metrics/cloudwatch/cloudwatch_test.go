package cloudwatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

func TestFixtureHeadroom(t *testing.T) {
	t.Parallel()
	buf := []byte(`{
	  "schema_version": "v0.1",
	  "samples": [
	    {"resource": "aws:opensearch:domain/search-lab", "metric": "JVMMemoryPressure", "current": 92},
	    {"resource": "aws:opensearch:domain/search-lab", "metric": "FreeStorageSpace", "current": 6000},
	    {"resource": "aws:rds:instance/other", "metric": "CPUUtilization", "current": 10}
	  ]
	}`)
	f, err := ParseFixture(buf, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	domain := model.Resource{ID: "aws:opensearch:domain/search-lab", Provider: "aws", Kind: "opensearch:domain", Name: "search-lab"}
	limits, err := f.Headroom(context.Background(), []model.Resource{domain})
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 2 {
		t.Fatalf("limits = %#v", limits)
	}
	byName := map[string]model.Limit{}
	for _, limit := range limits {
		byName[limit.Metric] = limit
	}
	if byName["JVMMemoryPressure"].Headroom != -7 {
		t.Fatalf("jvm = %v", byName["JVMMemoryPressure"].Headroom)
	}
	if byName["FreeStorageSpace"].Headroom != -14480 || byName["FreeStorageSpace"].Direction != model.DirectionHigherBetter {
		t.Fatalf("storage = %#v", byName["FreeStorageSpace"])
	}
}

func TestFixtureRejects(t *testing.T) {
	t.Parallel()
	if _, err := ParseFixture([]byte(`{"schema_version":"v9"}`), Thresholds{}); err == nil {
		t.Fatal("version")
	}
	f, err := ParseFixture([]byte(`{"schema_version":"v0.1","samples":[{"resource":"aws:opensearch:domain/search-lab","metric":"NotAMetric","current":1}]}`), Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	domain := model.Resource{ID: "aws:opensearch:domain/search-lab", Provider: "aws", Kind: "opensearch:domain", Name: "search-lab"}
	if _, err := f.Headroom(context.Background(), []model.Resource{domain}); err == nil {
		t.Fatal("unknown metric")
	}
	if _, err := LoadFixture("missing.json", Thresholds{}); err == nil {
		t.Fatal("missing file")
	}
}

func TestCloudWatchClient(t *testing.T) {
	t.Parallel()
	fake := &fakeAPI{out: &cloudwatch.GetMetricDataOutput{
		MetricDataResults: []cwtypes.MetricDataResult{{
			Id:         awsString("m1"),
			Timestamps: []time.Time{time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC()},
			Values:     []float64{10, 92},
		}},
	}}
	rec := &memAudit{}
	client := NewClient(fake, Thresholds{}, rec)
	client.now = func() time.Time { return time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC) }
	domain := model.Resource{
		ID: "aws:opensearch:domain/search-lab", Provider: "aws", Kind: "opensearch:domain", Name: "search-lab",
		Attrs: map[string]string{"account_id": "000000000000"},
	}
	rds := model.Resource{ID: "aws:rds:instance/payments", Provider: "aws", Kind: "rds:instance", Name: "payments"}
	msk := model.Resource{ID: "aws:msk:cluster/events", Provider: "aws", Kind: "msk:cluster", Name: "events", Attrs: map[string]string{"broker_id": "1"}}
	limits, err := client.Headroom(context.Background(), []model.Resource{domain, rds, msk})
	if err != nil {
		t.Fatal(err)
	}
	if fake.in == nil || len(fake.in.MetricDataQueries) == 0 {
		t.Fatal("no query")
	}
	sawGet := false
	for _, q := range fake.in.MetricDataQueries {
		if q.MetricStat == nil || q.MetricStat.Metric == nil || q.MetricStat.Metric.MetricName == nil {
			t.Fatal("empty metric")
		}
		if *q.MetricStat.Metric.MetricName == "JVMMemoryPressure" {
			sawGet = true
		}
		name := *q.MetricStat.Metric.MetricName
		for _, banned := range []string{"Put", "Delete", "Create", "Update"} {
			if len(name) >= len(banned) && name[:len(banned)] == banned {
				t.Fatalf("mutating metric name %s", name)
			}
		}
	}
	if !sawGet {
		t.Fatal("JVMMemoryPressure was not queried")
	}
	if len(limits) != 1 || limits[0].Metric != "JVMMemoryPressure" || limits[0].Current != 92 {
		t.Fatalf("limits = %#v", limits)
	}
	if len(rec.events) != 1 || rec.events[0] == "" {
		t.Fatalf("audit = %v", rec.events)
	}
}

func TestCloudWatchEmptyAndError(t *testing.T) {
	t.Parallel()
	client := NewClient(&fakeAPI{out: &cloudwatch.GetMetricDataOutput{}}, DefaultThresholds(), nil)
	limits, err := client.Headroom(context.Background(), []model.Resource{{
		ID: "k8s:deployment/atlas/atlas-indexer", Provider: "k8s", Kind: "deployment", Name: "atlas/atlas-indexer",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(limits) != 0 {
		t.Fatalf("%#v", limits)
	}
	failing := NewClient(&fakeAPI{err: errors.New("boom")}, Thresholds{}, nil)
	if _, err := failing.Headroom(context.Background(), []model.Resource{{
		ID: "aws:rds:instance/payments", Provider: "aws", Kind: "rds:instance", Name: "payments",
	}}); err == nil {
		t.Fatal("api error")
	}
	if _, err := (&Client{}).Headroom(context.Background(), nil); err == nil {
		t.Fatal("nil api")
	}
}

func TestThresholdDefaults(t *testing.T) {
	t.Parallel()
	got := (Thresholds{OpenSearchJVMMemoryPressure: 70}).WithDefaults()
	if got.OpenSearchJVMMemoryPressure != 70 || got.MSKDiskUsed != 85 {
		t.Fatalf("%#v", got)
	}
}

type fakeAPI struct {
	out *cloudwatch.GetMetricDataOutput
	err error
	in  *cloudwatch.GetMetricDataInput
}

func (f *fakeAPI) GetMetricData(_ context.Context, params *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	f.in = params
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

type memAudit struct{ events []string }

func (m *memAudit) Record(service, operation, target string) error {
	m.events = append(m.events, service+" "+operation+" "+target)
	return nil
}

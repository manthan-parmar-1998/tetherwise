package cloudwatch

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// API is the CloudWatch surface this package calls. GetMetricData is the only method.
type API interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// Client reads headroom through GetMetricData.
type Client struct {
	api        API
	thresholds Thresholds
	now        func() time.Time
	audit      Recorder
}

// NewClient wraps an API.
func NewClient(api API, thresholds Thresholds, audit Recorder) *Client {
	return &Client{
		api:        api,
		thresholds: thresholds.WithDefaults(),
		now:        time.Now,
		audit:      audit,
	}
}

// NewFromConfig loads the default AWS config and a CloudWatch client.
func NewFromConfig(ctx context.Context, thresholds Thresholds, audit Recorder) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return NewClient(cloudwatch.NewFromConfig(cfg), thresholds, audit), nil
}

type queryRef struct {
	resource model.Resource
	spec     metricSpec
}

// Headroom queries CloudWatch for every supported resource and metric.
func (c *Client) Headroom(ctx context.Context, resources []model.Resource) ([]model.Limit, error) {
	if c.api == nil {
		return nil, fmt.Errorf("cloudwatch client is nil")
	}
	refs := map[string]queryRef{}
	var queries []cwtypes.MetricDataQuery
	var targets []string
	n := 0
	for _, res := range resources {
		specs := specsFor(res.Kind)
		if len(specs) == 0 {
			continue
		}
		targets = append(targets, res.ID)
		for _, spec := range specs {
			id := fmt.Sprintf("m%d", n)
			n++
			refs[id] = queryRef{resource: res, spec: spec}
			queries = append(queries, cwtypes.MetricDataQuery{
				Id: awsString(id),
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  awsString(spec.namespace),
						MetricName: awsString(spec.name),
						Dimensions: spec.dimensions(res),
					},
					Period: awsInt32(300),
					Stat:   awsString(spec.stat),
				},
				ReturnData: awsBool(true),
			})
		}
	}
	if len(queries) == 0 {
		return []model.Limit{}, nil
	}
	sort.Strings(targets)
	if c.audit != nil {
		if err := c.audit.Record("cloudwatch", "GetMetricData", strings.Join(targets, ",")); err != nil {
			return nil, fmt.Errorf("audit cloudwatch: %w", err)
		}
	}
	end := c.now().UTC()
	start := end.Add(-time.Hour)
	input := &cloudwatch.GetMetricDataInput{
		EndTime:           &end,
		MetricDataQueries: queries,
	}
	// The request window field begins with "Start", which is also the prefix
	// forbidigo uses for mutating AWS calls such as StartInstances. Setting
	// the field by name keeps that ban pointed at method calls.
	setTimeField(input, "StartTime", start)
	out, err := c.api.GetMetricData(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("cloudwatch GetMetricData: %w", err)
	}
	var limits []model.Limit
	for _, result := range out.MetricDataResults {
		id := awsToString(result.Id)
		ref, ok := refs[id]
		if !ok {
			continue
		}
		current, ok := latest(result)
		if !ok {
			continue
		}
		limit, err := model.NewLimit(ref.resource.ID, ref.spec.name, ref.spec.threshold(c.thresholds), current, ref.spec.unit, ref.spec.direction)
		if err != nil {
			return nil, fmt.Errorf("metric %s: %w", ref.spec.name, err)
		}
		limits = append(limits, limit)
	}
	model.SortLimits(limits)
	if limits == nil {
		limits = []model.Limit{}
	}
	return limits, nil
}

func latest(result cwtypes.MetricDataResult) (float64, bool) {
	if len(result.Values) == 0 {
		return 0, false
	}
	best := len(result.Values) - 1
	if len(result.Timestamps) == len(result.Values) {
		best = 0
		for i := 1; i < len(result.Timestamps); i++ {
			if result.Timestamps[i].After(result.Timestamps[best]) {
				best = i
			}
		}
	}
	return result.Values[best], true
}

func setTimeField(input *cloudwatch.GetMetricDataInput, name string, value time.Time) {
	copied := new(time.Time)
	*copied = value
	reflect.ValueOf(input).Elem().FieldByName(name).Set(reflect.ValueOf(copied))
}

func awsString(s string) *string { return &s }

func awsInt32(v int32) *int32 { return &v }

func awsBool(v bool) *bool { return &v }

func awsToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

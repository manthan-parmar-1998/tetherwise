package cloudwatch

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

type metricSpec struct {
	kinds      []string
	namespace  string
	name       string
	stat       string
	unit       string
	direction  model.Direction
	threshold  func(Thresholds) float64
	dimensions func(model.Resource) []cwtypes.Dimension
}

func catalog() []metricSpec {
	return []metricSpec{
		{
			kinds: []string{"opensearch:domain"}, namespace: "AWS/ES", name: "FreeStorageSpace",
			stat: "Minimum", unit: "Megabytes", direction: model.DirectionHigherBetter,
			threshold:  func(t Thresholds) float64 { return t.OpenSearchFreeStorageSpaceMB },
			dimensions: openSearchDimensions,
		},
		{
			kinds: []string{"opensearch:domain"}, namespace: "AWS/ES", name: "JVMMemoryPressure",
			stat: "Maximum", unit: "Percent", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.OpenSearchJVMMemoryPressure },
			dimensions: openSearchDimensions,
		},
		{
			kinds: []string{"opensearch:domain"}, namespace: "AWS/ES", name: "CPUUtilization",
			stat: "Maximum", unit: "Percent", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.OpenSearchCPUUtilization },
			dimensions: openSearchDimensions,
		},
		{
			kinds: []string{"opensearch:domain"}, namespace: "AWS/ES", name: "ClusterStatus.red",
			stat: "Maximum", unit: "Count", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.OpenSearchClusterStatusRed },
			dimensions: openSearchDimensions,
		},
		{
			kinds: []string{"rds:instance"}, namespace: "AWS/RDS", name: "CPUUtilization",
			stat: "Maximum", unit: "Percent", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.RDSCPUUtilization },
			dimensions: func(r model.Resource) []cwtypes.Dimension { return dims("DBInstanceIdentifier", r.Name) },
		},
		{
			kinds: []string{"rds:instance"}, namespace: "AWS/RDS", name: "FreeableMemory",
			stat: "Minimum", unit: "Bytes", direction: model.DirectionHigherBetter,
			threshold:  func(t Thresholds) float64 { return t.RDSFreeableMemoryBytes },
			dimensions: func(r model.Resource) []cwtypes.Dimension { return dims("DBInstanceIdentifier", r.Name) },
		},
		{
			kinds: []string{"rds:instance"}, namespace: "AWS/RDS", name: "DatabaseConnections",
			stat: "Maximum", unit: "Count", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.RDSDatabaseConnections },
			dimensions: func(r model.Resource) []cwtypes.Dimension { return dims("DBInstanceIdentifier", r.Name) },
		},
		{
			kinds: []string{"rds:instance"}, namespace: "AWS/RDS", name: "FreeStorageSpace",
			stat: "Minimum", unit: "Bytes", direction: model.DirectionHigherBetter,
			threshold:  func(t Thresholds) float64 { return t.RDSFreeStorageSpaceBytes },
			dimensions: func(r model.Resource) []cwtypes.Dimension { return dims("DBInstanceIdentifier", r.Name) },
		},
		{
			kinds: []string{"msk:cluster"}, namespace: "AWS/Kafka", name: "KafkaDataLogsDiskUsed",
			stat: "Maximum", unit: "Percent", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.MSKDiskUsed },
			dimensions: mskDimensions,
		},
		{
			kinds: []string{"msk:cluster"}, namespace: "AWS/Kafka", name: "CpuUser",
			stat: "Maximum", unit: "Percent", direction: model.DirectionLowerBetter,
			threshold:  func(t Thresholds) float64 { return t.MSKCPUUser },
			dimensions: mskDimensions,
		},
	}
}

func specsFor(kind string) []metricSpec {
	var out []metricSpec
	for _, spec := range catalog() {
		for _, k := range spec.kinds {
			if k == kind {
				out = append(out, spec)
			}
		}
	}
	return out
}

func specByName(kind, name string) (metricSpec, bool) {
	for _, spec := range specsFor(kind) {
		if spec.name == name {
			return spec, true
		}
	}
	return metricSpec{}, false
}

func openSearchDimensions(res model.Resource) []cwtypes.Dimension {
	client := res.Attrs["client_id"]
	if client == "" {
		client = res.Attrs["account_id"]
	}
	return dims("DomainName", res.Name, "ClientId", client)
}

func mskDimensions(res model.Resource) []cwtypes.Dimension {
	return dims("Cluster Name", res.Name, "Broker ID", res.Attrs["broker_id"])
}

func dims(pairs ...string) []cwtypes.Dimension {
	var out []cwtypes.Dimension
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			continue
		}
		out = append(out, cwtypes.Dimension{Name: aws.String(pairs[i]), Value: aws.String(pairs[i+1])})
	}
	return out
}

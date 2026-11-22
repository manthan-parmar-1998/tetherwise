package cloudwatch

import (
	"context"

	"github.com/manthan-parmar-1998/tetherwise/internal/model"
)

// Provider reads headroom for a set of resources.
type Provider interface {
	Headroom(ctx context.Context, resources []model.Resource) ([]model.Limit, error)
}

// Recorder receives one entry per external read.
type Recorder interface {
	Record(service, operation, target string) error
}

// Thresholds are the limits headroom is compared against.
// A zero value means "use the default" via WithDefaults.
type Thresholds struct {
	OpenSearchFreeStorageSpaceMB float64 `mapstructure:"opensearch_free_storage_space_mb"`
	OpenSearchJVMMemoryPressure  float64 `mapstructure:"opensearch_jvm_memory_pressure"`
	OpenSearchCPUUtilization     float64 `mapstructure:"opensearch_cpu_utilization"`
	OpenSearchClusterStatusRed   float64 `mapstructure:"opensearch_cluster_status_red"`
	RDSCPUUtilization            float64 `mapstructure:"rds_cpu_utilization"`
	RDSFreeableMemoryBytes       float64 `mapstructure:"rds_freeable_memory_bytes"`
	RDSDatabaseConnections       float64 `mapstructure:"rds_database_connections"`
	RDSFreeStorageSpaceBytes     float64 `mapstructure:"rds_free_storage_space_bytes"`
	MSKDiskUsed                  float64 `mapstructure:"msk_disk_used"`
	MSKCPUUser                   float64 `mapstructure:"msk_cpu_user"`
}

// DefaultThresholds returns the v0.1 limits.
//
// OpenSearch free storage is a 20 GiB floor, in megabytes, matching the
// CloudWatch unit. JVM memory pressure trips at 85 percent. RDS freeable
// memory floors at 256 MiB and free storage at 20 GiB. Database connections
// floor at 100, which is deliberately conservative and meant to be raised
// per instance. MSK disk-used and CPU trip at 85 and 80 percent.
func DefaultThresholds() Thresholds {
	return Thresholds{
		OpenSearchFreeStorageSpaceMB: 20480,
		OpenSearchJVMMemoryPressure:  85,
		OpenSearchCPUUtilization:     80,
		OpenSearchClusterStatusRed:   1,
		RDSCPUUtilization:            80,
		RDSFreeableMemoryBytes:       256 * 1024 * 1024,
		RDSDatabaseConnections:       100,
		RDSFreeStorageSpaceBytes:     20 * 1024 * 1024 * 1024,
		MSKDiskUsed:                  85,
		MSKCPUUser:                   80,
	}
}

// WithDefaults fills zero fields from DefaultThresholds.
func (t Thresholds) WithDefaults() Thresholds {
	d := DefaultThresholds()
	if t.OpenSearchFreeStorageSpaceMB == 0 {
		t.OpenSearchFreeStorageSpaceMB = d.OpenSearchFreeStorageSpaceMB
	}
	if t.OpenSearchJVMMemoryPressure == 0 {
		t.OpenSearchJVMMemoryPressure = d.OpenSearchJVMMemoryPressure
	}
	if t.OpenSearchCPUUtilization == 0 {
		t.OpenSearchCPUUtilization = d.OpenSearchCPUUtilization
	}
	if t.OpenSearchClusterStatusRed == 0 {
		t.OpenSearchClusterStatusRed = d.OpenSearchClusterStatusRed
	}
	if t.RDSCPUUtilization == 0 {
		t.RDSCPUUtilization = d.RDSCPUUtilization
	}
	if t.RDSFreeableMemoryBytes == 0 {
		t.RDSFreeableMemoryBytes = d.RDSFreeableMemoryBytes
	}
	if t.RDSDatabaseConnections == 0 {
		t.RDSDatabaseConnections = d.RDSDatabaseConnections
	}
	if t.RDSFreeStorageSpaceBytes == 0 {
		t.RDSFreeStorageSpaceBytes = d.RDSFreeStorageSpaceBytes
	}
	if t.MSKDiskUsed == 0 {
		t.MSKDiskUsed = d.MSKDiskUsed
	}
	if t.MSKCPUUser == 0 {
		t.MSKCPUUser = d.MSKCPUUser
	}
	return t
}

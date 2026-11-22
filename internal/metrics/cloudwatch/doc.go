// Package cloudwatch reads headroom for OpenSearch, RDS, and MSK.
//
// The CloudWatch implementation calls GetMetricData only. A fixture
// implementation replays recorded samples so checks run offline.
// PutMetricData and every other mutating AWS API are out of scope.
package cloudwatch

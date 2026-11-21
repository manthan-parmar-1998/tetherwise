// Package cloudwatch reads CloudWatch metrics for shared resources.
//
// TODO(manthan): Read headroom for OpenSearch, RDS, and MSK with the
// CloudWatch read APIs (GetMetricData, GetMetricStatistics, ListMetrics).
// Never call PutMetricData or any other mutating AWS API. The forbidigo
// rules in .golangci.yml reject those calls.
package cloudwatch

package optimizerutil

import "k8s.io/apimachinery/pkg/api/resource"

var (
	CPUUsageQuery = `avg_over_time(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s"}[1h])[1d:1h])`

	MemUsageQuery = `avg_over_time(container_memory_usage_bytes{namespace="%s", pod="%s"}[1d:1h])`

	CPURequestQuery = `sum(kube_pod_container_resource_requests_cpu_cores{namespace="%s", pod="%s"}) by (pod)`

	MemoryRequestQuery = `sum(kube_pod_container_resource_requests_memory_bytes{namespace="%s", pod="%s"}) by (pod)`

	PromQuerySlice = []string{CPUUsageQuery, MemUsageQuery}
)

const (
	CPUUsageAvgData = "cpu_usage_avg"
	MemUsageAvgData = "mem_usage_avg"
)

func getCPUValue(rs resource.Quantity) int64 {
	return rs.MilliValue()
}

func getMemoryValue(rs resource.Quantity) int64 {
	return rs.ScaledValue(resource.Mega)
}

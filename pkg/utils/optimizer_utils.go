package optimizerutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModel "github.com/prometheus/common/model"
	pod "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodRequest struct {
	Pod           string    `json:"pod"`
	Namespace     string    `json:"namespace"`
	CPURequest    int64     `json:"cpu_request"`
	MemoryRequest int64     `json:"memory_request"`
	CPULimit      int64     `json:"cpu_limit"`
	MemoryLimit   int64     `json:"memory_limit"`
	CPUUsage      int64     `json:"cpu_usage"`
	MemoryUsage   int64     `json:"memory_usage"`
	RequestTime   time.Time `json:"request_time"`
}

type resourceList struct {
	CPURequest    int64
	MemoryRequest int64
	CPULimit      int64
	MemoryLimit   int64
}

var promQueryMap = map[string]resource.Quantity{}

var setupLog = ctrl.Log.WithName("setup")

func GetPodData(ctx context.Context, namespacedName types.NamespacedName) error {
	setupLog.Info("Reconciling CostOptimizer", "name", "SetupWithManager")

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// List all pods in the cluster
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	prometheusClient, err := api.NewClient(api.Config{
		Address: "http://prometheus-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090",
	})
	if err != nil {
		setupLog.Error(err, "failed to create Prometheus client")
		return err
	}
	v1API := v1.NewAPI(prometheusClient)
	queryRange := v1.Range{
		Start: time.Now().Add(-1 * 24 * time.Hour),
		End:   time.Now(),
		Step:  time.Hour,
	}

	for _, pod := range pods.Items {
		setupLog.Info("Running Job for", "pod", pod.Name, "namespace", pod.Namespace)
		podResources := getResourceRequest(&pod)
		for _, query := range PromQuerySlice {
			setupLog.Info("Querying Prometheus", "query", query)

			result, err := promQueryRangeResult(ctx, v1API, queryRange, query, pod.Name, pod.Namespace)
			if err != nil {
				setupLog.Error(err, "failed to query Prometheus")
				return err
			}

			mapData := make(map[promModel.Time]promModel.SampleValue)
			var sum promModel.SampleValue
			var count int
			for _, val := range result.(promModel.Matrix) {
				for _, v := range val.Values {
					mapData[v.Timestamp] = v.Value
					sum += v.Value
					count++
				}
			}
			average := sum / promModel.SampleValue(count)
			setupLog.Info("Average value from Prometheus data", "average", average.String())
			if average.String() == "NaN" {
				setupLog.Info("No data available")
				continue
			}
			parsedAvg := resource.MustParse(average.String())
			promQueryMap[query] = parsedAvg
		}
		request := PodRequest{
			Pod:           pod.Name,
			Namespace:     pod.Namespace,
			CPURequest:    podResources.CPURequest,
			MemoryRequest: podResources.MemoryRequest,
			CPUUsage:      getCPUValue(promQueryMap[CPUUsageQuery]),
			MemoryUsage:   getMemoryValue(promQueryMap[MemUsageQuery]),
			CPULimit:      podResources.CPULimit,
			MemoryLimit:   podResources.MemoryLimit,
			RequestTime:   time.Now(),
		}
		err = createPodRequest(request)
		if err != nil {
			setupLog.Error(err, "failed to create pod request for pod %v namespace %v", pod.Name, pod.Namespace)
			return err
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

// nolint:lll
func promQueryRangeResult(ctx context.Context, promAPI v1.API, queryRange v1.Range, query, podName, podNamespace string) (result promModel.Value, err error) {
	query = fmt.Sprintf(query, podNamespace, podName)
	result, warnings, err := promAPI.QueryRange(ctx, query, queryRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus: %w", err)
	}

	if len(warnings) > 0 {
		log.FromContext(ctx).Info("Prometheus query warnings", "warnings", warnings)
		return nil, nil
	}
	return result, nil
}

func getResourceRequest(pod *pod.Pod) resourceList {
	var totalCPURequest, totalMemoryRequest, totalCPULimit, totalMemoryLimit resource.Quantity

	for _, container := range pod.Spec.Containers {
		requests := container.Resources.Requests
		limits := container.Resources.Limits

		totalCPURequest.Add(*requests.Cpu())
		totalMemoryRequest.Add(*requests.Memory())
		totalCPULimit.Add(*limits.Cpu())
		totalMemoryLimit.Add(*limits.Memory())
	}

	return resourceList{
		CPURequest:    totalCPURequest.MilliValue(),
		MemoryRequest: totalMemoryRequest.ScaledValue(resource.Mega),
		CPULimit:      totalCPULimit.MilliValue(),
		MemoryLimit:   totalMemoryLimit.ScaledValue(resource.Mega),
	}
}

func createPodRequest(request PodRequest) error {
	jsonReq, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	config, err := loadConfig(".")
	if err != nil {
		return fmt.Errorf("Failed to load config: %v\n", err)
	}

	setupLog.Info("Request to be sent to the API", "request", string(jsonReq))
	// nolint:lll
	uri := fmt.Sprintf("http://%s.%s.svc.cluster.local%s/api/v1/pod-insights", config.InsightsService, config.Namespace, config.HttpAddress)
	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewBuffer(jsonReq))
	if err != nil {
		return fmt.Errorf("failed to get New http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			setupLog.Error(err, "failed to close response body")
			return
		}
	}()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("failed to send request: %w", err)
	}
	return nil
}

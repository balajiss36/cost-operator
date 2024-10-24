package optimizerutil

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promModel "github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var setupLog = ctrl.Log.WithName("setup")

func GetPodData(ctx context.Context, namespacedName types.NamespacedName) error {
	setupLog.Info("Reconciling CostOptimizer", "name", "SetupWithManager")

	prometheusClient, err := api.NewClient(api.Config{
		Address: "http://prometheus-kube-prometheus-prometheus.monitoring.svc.cluster.local:9090",
		// RoundTripper: userAgentRoundTripper{name: "Client-Golang", rt: api.DefaultRoundTripper},
	})
	if err != nil {
		setupLog.Error(err, "failed to create Prometheus client")
		return err
	}

	v1api := v1.NewAPI(prometheusClient)
	rang := v1.Range{
		Start: time.Now().Add(-1 * 24 * time.Hour),
		End:   time.Now(),
		Step:  time.Hour,
	}
	// nolint:lll
	query := fmt.Sprintf(`avg_over_time(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s"}[1h])[1d:1h])`, namespacedName.Namespace, namespacedName.Name)
	// query := `rate(container_cpu_usage_seconds_total[1m])`
	setupLog.Info("Querying Prometheus", "query", query)
	result, warnings, err := v1api.QueryRange(ctx, query, rang)
	if err != nil {
		return fmt.Errorf("failed to query Prometheus: %w", err)
	}

	if len(warnings) > 0 {
		log.FromContext(ctx).Info("Prometheus query warnings", "warnings", warnings)
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

	setupLog.Info("Result from Prometheus", "resultType", result.Type(), "resultValue", result)
	setupLog.Info("Result from Prometheus", "processed ResultValue", mapData)
	setupLog.Info("Average value from Prometheus data", "average", average)

	// log.FromContext(ctx).Info("Result from Prometheus queries by Pod %v", result)
	return nil
}

/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	optimizerv1alpha1 "github.com/balajiss36/cost-operator/api/v1alpha1"
)

// CostOptimizerReconciler reconciles a CostOptimizer object
type CostOptimizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var setupLog = ctrl.Log.WithName("setup")

// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CostOptimizer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *CostOptimizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling CostOptimizer", "name", req.NamespacedName)

	var optimizer optimizerv1alpha1.CostOptimizer
	if err := r.Client.Get(ctx, req.NamespacedName, &optimizer); err != nil {
		logger.Error(err, "unable to fetch CostOptimizer")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err := r.GetPodData(ctx); err != nil {
		logger.Error(err, "failed to get pod data")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *CostOptimizerReconciler) GetPodData(ctx context.Context) error {
	prometheusClient, err := api.NewClient(api.Config{
		Address: "http://prometheus-server.default.svc:9090",
	})
	if err != nil {
		return fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	v1api := v1.NewAPI(prometheusClient)
	query := `avg(rate(container_cpu_usage_seconds_total{namespace="default"}[5m])) by (pod)`
	result, warnings, err := v1api.Query(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to query Prometheus: %w", err)
	}

	if len(warnings) > 0 {
		log.FromContext(ctx).Info("Prometheus query warnings", "warnings", warnings)
	}

	log.FromContext(ctx).Info("Result from Prometheus queries by Pod", "result", result.String())
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CostOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	setupLog.Info("Reconciling CostOptimizer", "name", "SetupWithManager")

	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1alpha1.CostOptimizer{}).
		// Owns(&corev1.Pod{}).
		WithLogConstructor(func(request *reconcile.Request) logr.Logger {
			return mgr.GetLogger()
		}).
		Complete(r)
}

// // For every pods, CostOptimizer would run
// func (r *CostOptimizerReconciler) GetAll(obj handler.MapObject) []ctrl.Request {
// }

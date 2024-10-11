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
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	corev1 "k8s.io/api/core/v1"
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
	log := ctrl.LoggerFrom(ctx)

	// TODO(user): your logic here
	// This is to watch your resource and perform some action if it is not in the desired state

	log.Info("triggered the Reconcile loop")

	var optimizer optimizerv1alpha1.CostOptimizer
	err := r.Client.Get(ctx, req.NamespacedName, &optimizer)
	if err != nil {
		log.Error(err, "unable to fetch CostOptimizer")
		return ctrl.Result{}, err
	}

	var list optimizerv1alpha1.CostOptimizerList
	err = r.Client.List(ctx, &list)
	if err != nil {
		log.Error(err, "unable to list CostOptimizer")
		return ctrl.Result{}, err
	}
	r.GetPodData()

	return ctrl.Result{}, nil
}

func (r *CostOptimizerReconciler) GetPodData() []reconcile.Request {
	prometheusClient, err := api.NewClient(api.Config{
		Address: "http://prometheus-server.default.svc:9090",
	})

	v1api := v1.NewAPI(prometheusClient)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err != nil {
		log.Log.Error(err, "Failed to create Prometheus client")
		return []ctrl.Request{}
	}

	// gvk, err := r.GroupVersionKindFor(we)
	// if err != nil {
	// 	log.Log.Error(err, "Failed to get GVK for resource")
	// 	return []ctrl.Request{}
	// }
	// log.Log.Info("GVK by Pod %v", gvk)

	query := `avg(rate(container_cpu_usage_seconds_total{namespace="default"}[5m])) by (pod)`
	result, warnings, err := v1api.Query(ctx, query, time.Now())
	if err != nil {
		log.Log.Error(err, "Failed to query Prometheus")
		return []ctrl.Request{}
	}

	if len(warnings) > 0 {
		log.Log.Info("Warnings: %v", warnings)
	}
	log.Log.Info("Result from Prometheus queries by Pod %v", result.String())
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CostOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1alpha1.CostOptimizer{}).
		Owns(&corev1.Pod{}).
		WithLogConstructor(func(request *reconcile.Request) logr.Logger {
			return mgr.GetLogger()
		}).
		// Watches(source.Kind("cache", &Type{&corev1.Pod{}}, handler.EnqueueRequestForOwner())).
		// Watches(
		// 	&source.Kind{Type: &corev1.Pod{}},
		// 	handler.EnqueueRequestsFromMapFunc(),
		// 	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		// ).
		Complete(r)
}

// // For every pods, CostOptimizer would run
// func (r *CostOptimizerReconciler) GetAll(obj handler.MapObject) []ctrl.Request {
// }

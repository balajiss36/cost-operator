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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	// This is to watch your resource and perform some action if it is not in the desired state
	var optimizer optimizerv1alpha1.CostOptimizer
	err := r.Client.Get(ctx, req.NamespacedName, &optimizer)
	if err != nil {
		log.Log.Error(err, "unable to fetch CostOptimizer")
		return ctrl.Result{}, err
	}

	var list optimizerv1alpha1.CostOptimizerList
	err = r.Client.List(ctx, &list)
	if err != nil {
		log.Log.Error(err, "unable to list CostOptimizer")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CostOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1alpha1.CostOptimizer{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestsFromMapFunc().
			Complete(r))
	// If you want to perform some action on the other related resources, you can add them here
}

// // For every pods, CostOptimizer would run
// func (r *CostOptimizerReconciler) GetAll(obj handler.MapObject) []ctrl.Request {
// }

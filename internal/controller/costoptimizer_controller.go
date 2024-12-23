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

	optimizerv1alpha1 "github.com/balajiss36/cost-operator/api/v1alpha1"
	optimizerutils "github.com/balajiss36/cost-operator/pkg/utils"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CostOptimizerReconciler reconciles a CostOptimizer object
type CostOptimizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *CostOptimizerReconciler) Now() time.Time { return time.Now() }

var (
	setupLog                = ctrl.Log.WithName("setup")
	scheduledTimeAnnotation = "batch.optimizer.dev.builder.io/scheduled-at"
	mostRecentTime          *time.Time
)

//nolint:lll
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups="",resources=pods,verbs=get

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
	setupLog.Info("Reconciling CostOptimizer", "name", "SetupWithManager")

	var optimizer optimizerv1alpha1.CostOptimizer
	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	// Get the Optimizer Object
	if err := r.Client.Get(ctx, req.NamespacedName, &optimizer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scheduledTimeForJob, err := getScheduledTimeForJob(&optimizer)
	if err != nil {
		setupLog.Error(err, "unable to parse schedule time for job", "job", &optimizer)
	}
	if scheduledTimeForJob != nil {
		if mostRecentTime == nil || mostRecentTime.Before(*scheduledTimeForJob) {
			mostRecentTime = scheduledTimeForJob
		}
	}

	if mostRecentTime != nil {
		optimizer.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		optimizer.Status.LastScheduleTime = nil
	}
	setupLog.Info("Reconciling CostOptimizer", "scheduletime for Run", scheduledTimeForJob)

	missedRun, nextRun, err := getNextSchedule(&optimizer, r.Now())
	if err != nil {
		setupLog.Error(err, "unable to figure out schedule")
		return ctrl.Result{}, nil
	}
	setupLog.Info("Reconciling CostOptimizer", "next Run", nextRun)

	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())}

	if missedRun.IsZero() {
		setupLog.Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	currentTime := time.Now()
	setupLog.Info("Reconciling CostOptimizer", "current Run", currentTime)
	err = optimizerutils.GetPodData(ctx, namespacedName)
	if err != nil {
		setupLog.Error(err, "failed to get pod data")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: time.Second * 10,
		}, err
	}
	optimizer.CreationTimestamp.Time = currentTime
	optimizer.Annotations[scheduledTimeAnnotation] = currentTime.Format(time.RFC3339)
	setupLog.Info("Reconciling CostOptimizer", "scheduletime for Run", optimizer.Annotations[scheduledTimeAnnotation])
	setupLog.Info("Reconciling CostOptimizer", "next Schedule Run", scheduledResult)
	return scheduledResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CostOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	createPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&optimizerv1alpha1.CostOptimizer{}).
		Owns(&corev1.Pod{}).
		WithEventFilter(createPredicate).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

func getScheduledTimeForJob(job *optimizerv1alpha1.CostOptimizer) (*time.Time, error) {
	timeRaw := job.Annotations[scheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return nil, nil
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return nil, err
	}
	return &timeParsed, nil
}

func getNextSchedule(optJob *optimizerv1alpha1.CostOptimizer, now time.Time) (lastMissed, next time.Time, err error) {
	sched, err := cron.ParseStandard(optJob.Spec.Schedule)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("Unparseable schedule %q: %v", optJob.Spec.Schedule, err)
	}

	setupLog.Info("Reconciling CostOptimizer", "schedule", sched)

	var earliestTime time.Time
	if optJob.Status.LastScheduleTime != nil {
		earliestTime = optJob.Status.LastScheduleTime.Time
		setupLog.Info("Reconciling CostOptimizer", "earliestTime from last schedule", earliestTime)
	} else {
		earliestTime = optJob.ObjectMeta.CreationTimestamp.Time
	}
	// If the earliest time is in the future, we should wait until then.
	if earliestTime.After(now) {
		setupLog.Info("Reconciling CostOptimizer", "earliestTime in loop", earliestTime)
		return time.Time{}, sched.Next(now), nil
	}
	starts := 0
	// loop is detemine missed start time and check if it it is not overwhelmed with a lot of missed times.
	for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
		setupLog.Info("Reconciling CostOptimizer", "newsched time inside loop", sched.Next(t))
		lastMissed = t
		starts++
		if starts > 100 {
			return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100)")
		}
	}
	return lastMissed, sched.Next(now), nil
}

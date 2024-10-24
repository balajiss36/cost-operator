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
	"net/http"
	"time"

	optimizerv1alpha1 "github.com/balajiss36/cost-operator/api/v1alpha1"
	optimizerutils "github.com/balajiss36/cost-operator/pkg/utils"
	"github.com/robfig/cron"
	kbatch "k8s.io/api/batch/v1"
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
	jobOwnerKey             = ".metadata.controller"
	scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
	activeJobs              []*kbatch.Job
	successfulJobs          []*kbatch.Job
	failedJobs              []*kbatch.Job
	mostRecentTime          *time.Time
)

//nolint:lll
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=optimizer.dev.builder,resources=costoptimizers/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

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
	// logger := log.FromContext(ctx)
	setupLog.Info("Reconciling CostOptimizer", "name", "SetupWithManager")

	var optimizer optimizerv1alpha1.CostOptimizer
	namespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	// Get the Optimizer Object
	setupLog.Info("Reconciling CostOptimizer", "name", namespacedName)
	if err := r.Client.Get(ctx, req.NamespacedName, &optimizer); err != nil {
		setupLog.Error(err, "unable to fetch CostOptimizer")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if optimizer.Status.Active == "Running" {
		setupLog.Info("Reconciling CostOptimizer", "name", "Action job already exists")
	}

	scheduledTimeForJob, err := getScheduledTimeForJob(&optimizer)
	if err != nil {
		setupLog.Error(err, "unable to parse schedule time for job", "job", &optimizer)
		// continue
	}
	if scheduledTimeForJob != nil {
		if mostRecentTime == nil || mostRecentTime.Before(*scheduledTimeForJob) {
			mostRecentTime = scheduledTimeForJob
		}
	}
	// }

	if mostRecentTime != nil {
		optimizer.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		optimizer.Status.LastScheduleTime = nil
	}

	missedRun, nextRun, err := getNextSchedule(&optimizer, r.Now())
	if err != nil {
		setupLog.Error(err, "unable to figure out CronJob schedule")
		return ctrl.Result{}, nil
	}
	setupLog.Info("Reconciling CostOptimizer", "next Run", nextRun)

	setupLog.Info("Reconciling CostOptimizer", "scheduletime for Run", scheduledTimeForJob)

	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())}

	if missedRun.IsZero() {
		setupLog.Info("no upcoming scheduled times, sleeping until next")
		return ctrl.Result{}, nil
	}

	currentTime := time.Now()
	setupLog.Info("Reconciling CostOptimizer", "current Run", currentTime)
	if nextRun.After(*scheduledTimeForJob) {
		setupLog.Info("Reconciling CostOptimizer", "next Run", "GetPodData")

		err := optimizerutils.GetPodData(ctx, namespacedName)
		if err != nil {
			setupLog.Error(err, "failed to get pod data")
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 10,
			}, err
		}
		optimizer.CreationTimestamp.Time = currentTime
		optimizer.Status.Active = "Running"
		optimizer.Annotations[scheduledTimeAnnotation] = currentTime.Format(time.RFC3339)
	}
	setupLog.Info("Reconciling CostOptimizer", "name", "Action job created")
	setupLog.Info("Reconciling CostOptimizer", "next Schedule Run", scheduledResult)
	return scheduledResult, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CostOptimizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// secretResource := &source.Kind{Type: &corev1.Secret{TypeMeta: metav1.TypeMeta{APIVersion: APIVersion, Kind: SecretKind}}}

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
		WithEventFilter(createPredicate).
		// Watches(&corev1.Pod{}, &handler.Funcs{CreateFunc: r.GetAll}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}

// // For every pods, CostOptimizer would run
// func (r *CostOptimizerReconciler) GetAll(obj handler.MapObject) []ctrl.Request {
// }

// func (r *CostOptimizerReconciler) GetAll(context.Context, event.TypedCreateEvent[client.Object], workqueue.TypedRateLimitingInterface[reconcile.Request]) {
// 	// Implement logic to handle Pod events and return reconciliation requests
// 	// For example, you can return a list of requests to reconcile all CostOptimizers
// 	var requests []reconcile.Request
// 	var list optimizerv1alpha1.CostOptimizerList
// 	ctx := context.Background()

// 	if err := r.Client.List(ctx, &list); err != nil {
// 		log.Log.Error(err, "unable to list CostOptimizers")
// 		return
// 	}

// 	for _, item := range list.Items {
// 		requests = append(requests, reconcile.Request{
// 			NamespacedName: types.NamespacedName{
// 				Name:      item.Name,
// 				Namespace: item.Namespace,
// 			},
// 		})
// 	}

// 	return
// }

var client1 = http.Client{
	Timeout: 2 * time.Second,
}

func Ping(domain string) (int, error) {
	url := "http://" + domain
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client1.Do(req)
	if err != nil {
		return 0, err
	}
	err = resp.Body.Close()
	if err != nil {
		return 0, err
	}
	return resp.StatusCode, nil
}

func getScheduledTimeForJob(job *optimizerv1alpha1.CostOptimizer) (*time.Time, error) {
	timeRaw := job.Annotations[scheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		timeParsed, _ := time.Parse(time.RFC3339, "2024-10-24T15:32:00+05:30")
		return &timeParsed, nil
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

	var earliestTime time.Time
	if optJob.Status.LastScheduleTime != nil {
		earliestTime = optJob.Status.LastScheduleTime.Time
	} else {
		earliestTime = optJob.ObjectMeta.CreationTimestamp.Time
	}
	if earliestTime.After(now) {
		return time.Time{}, sched.Next(now), nil
	}

	starts := 0
	for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
		lastMissed = t
		starts++
		if starts > 100 {
			return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100)")
		}
	}
	return lastMissed, sched.Next(now), nil
}

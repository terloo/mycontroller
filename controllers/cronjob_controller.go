/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/robfig/cron"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "mycontroller/api/v1"
)

var (
	// 调度时间Cron的Annotation
	scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
	// 通过job索引cronJob的key
	jobOwnerKey = ".metadata.controller"
	// apiGroupVersion
	apiGVStr = batchv1.GroupVersion.String()
)

// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clock
}

// 给controller授RBAC权限
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Request只有一个操作对象的名字，需要自己使用client获取全部对象信息

	log := log.FromContext(ctx)

	// 1. 通过名字从Client获取CronJob对象
	var cronJob batchv1.CronJob
	if err := (r.Client.Get(ctx, req.NamespacedName, &cronJob)); err != nil {
		log.Error(err, "unabel to fetch CronJob")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. 列出所有活动的Job，并且更新它们的状态
	var childJobs kbatch.JobList
	if err := r.Client.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		log.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}
	// 分类所有的Job
	var activeJobs []*kbatch.Job
	var successfulJobs []*kbatch.Job
	var failedJobs []*kbatch.Job
	var mostRecentTime *time.Time // 最近一次运行时间

	isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}
		return false, ""
	}

	getScheduleTimeFromJob := func(job *kbatch.Job) (*time.Time, error) {
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

	for i, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "":
			activeJobs = append(activeJobs, &childJobs.Items[i])
		case kbatch.JobFailed:
			failedJobs = append(failedJobs, &childJobs.Items[i])
		case kbatch.JobComplete:
			successfulJobs = append(successfulJobs, &childJobs.Items[i])
		}

		scheduleTimeForJob, err := getScheduleTimeFromJob(&job)
		if err != nil {
			log.Error(err, "unable to parse schedule time for child job", "job", &job)
			continue
		}
		if scheduleTimeForJob != nil {
			if mostRecentTime == nil {
				mostRecentTime = scheduleTimeForJob
			} else if mostRecentTime.Before(*scheduleTimeForJob) {
				mostRecentTime = scheduleTimeForJob
			}
		}
	}

	if mostRecentTime != nil {
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		cronJob.Status.LastScheduleTime = nil
	}
	cronJob.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := reference.GetReference(r.Scheme, activeJob)
		if err != nil {
			log.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		cronJob.Status.Active = append(cronJob.Status.Active, jobRef)
	}

	log.V(1).Info("job count", "active jobs", len(activeJobs), "success jobs", len(successfulJobs), "failed jobs", len(failedJobs))

	// 对Status的更新将会忽略对Spec的修改
	if err := r.Status().Update(ctx, &cronJob); err != nil {
		log.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, nil
	}

	// 3. 根据HistoryLimit来清理旧Job
	if cronJob.Spec.FailedJobHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[i].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
		})
		for i, job := range failedJobs {
			if int32(i) >= int32(len(failedJobs))-*cronJob.Spec.FailedJobHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete old failed job", "job", job)
			} else {
				log.V(0).Info("deleted failed job", "job", job)
			}
		}
	}

	if cronJob.Spec.SuccessfulJobHistoryLimit != nil {
		sort.Slice(successfulJobs, func(i, j int) bool {
			if successfulJobs[i].Status.StartTime == nil {
				return successfulJobs[j].Status.StartTime != nil
			}
			return successfulJobs[i].Status.StartTime.Before(successfulJobs[j].Status.StartTime)
		})
		for i, job := range successfulJobs {
			if int32(i) >= int32(len(successfulJobs))-*cronJob.Spec.SuccessfulJobHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unabel to delete old successful job", "job", job)
			} else {
				log.V(0).Info("delete old successful job", "job", job)
			}
		}
	}

	// 4. 检查Cron是否暂停，如果暂停，则不应该运行新的Job
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		log.V(0).Info("cronJob suspended, skipping")
		return ctrl.Result{}, nil
	}

	// 5. 获取下一次运行的时间，以及是否有未执行的Job
	getNextSchedule := func(cronJob *batchv1.CronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Unparseabel schedule %q: %v", cronJob.Spec.Schedule, err)
		}
		var earliestTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			earliestTime = cronJob.Status.LastScheduleTime.Time
		} else {
			earliestTime = cronJob.ObjectMeta.CreationTimestamp.Time
		}
		if cronJob.Spec.StartingDeadlineSeconds != nil {
			schedulingDeadline := now.Add(-time.Second * time.Duration(*cronJob.Spec.StartingDeadlineSeconds))
			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}
		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t
			starts++
			if starts > 100 {
				// 无法获得最近的时间，返回空切片
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start time (>100), Set or decrease .spec.startingDeadlineSeconds or check clock skew.")
			}
		}
		return lastMissed, sched.Next(now), nil
	}
	// 获取下一次运行job的时间，或者错过的
	missedRun, nextRun, err := getNextSchedule(&cronJob, r.Now())
	if err != nil {
		log.Error(err, "unable to figure out CronJob schedule")
		return ctrl.Result{}, nil
	}
	// 准备最终结果，在需要运行时运行
	scheduleResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())}
	log = log.WithValues("now", r.Now(), "next run", nextRun)

	// 6. 如果在该调度时则运行一个新的Job
	if missedRun.IsZero() {
		log.V(1).Info("no upcoming scheduled times, sleeping until next")
	}

	// 保证运行Job不会太迟
	log = log.WithValues("current run ", missedRun)
	tooLate := false
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second).Before(r.Now())
	}
	if tooLate {
		log.V(1).Info("missed starting deadline for last run, sleeping till next")
		// TODO(directxman12): events
		return scheduleResult, nil
	}

	// 根据ConcurrencyPolicy来处理新老Job
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent && len(activeJobs) > 0 {
		log.V(1).Info("concurrencyPolicy block concurrent run, skipping", "num active", len(activeJobs))
	}
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			if err := r.Client.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				log.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	// 创建新的Job
	constructNewJobFromCronJob := func(cronJob *batchv1.CronJob, scheduleTime time.Time) (*kbatch.Job, error) {
		// job名字
		name := fmt.Sprintf("%s-%d", cronJob.Name, scheduleTime.Unix())

		job := &kbatch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   cronJob.Namespace,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
		}
		for k, v := range cronJob.Spec.JobTemplate.Annotations {
			job.Annotations[k] = v
		}
		for k, v := range cronJob.Spec.JobTemplate.Labels {
			job.ObjectMeta.Labels[k] = v
		}
		job.Annotations[scheduledTimeAnnotation] = scheduleTime.Format(time.RFC3339)
		if err := ctrl.SetControllerReference(cronJob, job, r.Scheme); err != nil {
			return nil, err
		}
		return job, nil

	}
	job, err := constructNewJobFromCronJob(&cronJob, missedRun)
	if err != nil {
		log.Error(err, "unable to construct job from template")
		return scheduleResult, nil
	}
	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "unable to create Job for Cronjob", "job", job)
		return ctrl.Result{}, nil
	}
	log.V(1).Info("created Job for CronJob run", "job", job)

	// 7. 如果没有运行中的job或者不是该调度的时间，重新开始排队
	return scheduleResult, nil
}

// 一个获取当前时间的接口，使用接口方便测试
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (_ realClock) Now() time.Time {
	return time.Now()
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, jobOwnerKey, func(rawObject client.Object) []string {
		job := rawObject.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		if owner.APIVersion != apiGVStr || owner.Kind != "CronJob" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}
	// 注册该reconciler到manager
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Complete(r)
}

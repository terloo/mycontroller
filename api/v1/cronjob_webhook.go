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

package v1

import (
	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// log is for logging in this package.
var cronjoblog = logf.Log.WithName("cronjob-resource")

func (r *CronJob) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-batch-tutorial-kubebuilder-io-v1-cronjob,mutating=true,failurePolicy=fail,sideEffects=None,groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=create;update,versions=v1,name=mcronjob.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &CronJob{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *CronJob) Default() {
	cronjoblog.Info("default", "name", r.Name)

	if r.Spec.ConcurrencyPolicy == "" {
		r.Spec.ConcurrencyPolicy = AllowConcurrent
	}
	if r.Spec.Suspend == nil {
		r.Spec.Suspend = new(bool)
	}
	if r.Spec.SuccessfulJobHistoryLimit == nil {
		r.Spec.SuccessfulJobHistoryLimit = new(int32)
		*r.Spec.SuccessfulJobHistoryLimit = 3
	}
	if r.Spec.FailedJobHistoryLimit == nil {
		r.Spec.FailedJobHistoryLimit = new(int32)
		*r.Spec.FailedJobHistoryLimit = 1
	}

}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-batch-tutorial-kubebuilder-io-v1-cronjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=create;update,versions=v1,name=vcronjob.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &CronJob{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *CronJob) ValidateCreate() error {
	cronjoblog.Info("validate create", "name", r.Name)

	return r.ValidateCronJob()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *CronJob) ValidateUpdate(old runtime.Object) error {
	cronjoblog.Info("validate update", "name", r.Name)

	return r.ValidateCronJob()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CronJob) ValidateDelete() error {
	cronjoblog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}

func (r *CronJob) ValidateCronJob() error {
	var allErrs field.ErrorList
	if err := r.validateCronJobName(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateCronJobSpec(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(schema.GroupKind{Group: "batch.tutorial.kubebuilder.io", Kind: "CronJob"}, r.Name, allErrs)
}

func (r *CronJob) validateCronJobSpec() *field.Error {
	return validateScheduleFormat(r.Spec.Schedule, field.NewPath("spec").Child("schedule"))
}

func (r *CronJob) validateCronJobName() *field.Error {
	if len(r.ObjectMeta.Name) > validationutils.DNS1035LabelMaxLength-11 {
		return field.Invalid(field.NewPath("metadata").Child("name"), r.Name, "must be no more than 52 characters")
	}
	return nil
}

func validateScheduleFormat(schedule string, fldPath *field.Path) *field.Error {
	if _, err := cron.ParseStandard(schedule); err != nil {
		return field.Invalid(fldPath, schedule, err.Error())
	}
	return nil
}

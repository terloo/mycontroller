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
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CronJobSpec defines the desired state of CronJob
type CronJobSpec struct {
	//+kubebuilder:validation:MinLength=0
	// 调度时间 Cron表达式
	Schedule string `json:"schedule"`

	//+kubebuilder:validation:Minimum=0
	//+optional
	// Deadline时间
	StartingDeadlineSeconds *int64 `json:"startingDeadlineSeconds,omitempty"`

	//+optional
	// 指定如何处理Job的并发执行
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	//+optional
	// 指定是否暂停Cron的执行，默认为false
	Suspend *bool `json:"suspend,omitempty"`

	//Job模板
	JobTemplate batchv1beta1.JobTemplateSpec `json:"jobTemplate,omitempty"`

	//+kubebuilder:validation:Minimum=0
	//+optional
	// 成功的Job数限制，区分未指定和0
	SuccessfulJobHistoryLimit *int32 `json:"successfulJobHistoryLimit,omitempty"`

	//+kubebuilder:validation:Minimum=0
	//+optional
	// 失败的Job数限制，区分未指定和0
	FailedJobHistoryLimit *int32 `json:"failedJobHistoryLimit,omitempty"`
}

// CronJobStatus defines the observed state of CronJob
type CronJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	//+optional
	// 当前正在运行的Job的指针
	Active []*corev1.ObjectReference `json:"active,omitempty"`

	//+optional
	// 上次成功执行的Job的时间
	LastScheduleTime *metav1.Time `json:"last_schedule_time,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CronJob is the Schema for the cronjobs API
type CronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronJobSpec   `json:"spec,omitempty"`
	Status CronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CronJobList contains a list of CronJob
type CronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronJob `json:"items"`
}

type ConcurrencyPolicy string

//+kubebuilder:validation:Enum=Allow;Forbid;Replace
// ConcurrencyPolicy 描述了在前一个Job未完成时如何执行后一个Job，默认为AllowConcurrent
// "Always"(default)：允许并发执行
// "Forbid"：禁止，在上个Job执行完毕后再执行下个Job
// "Replace"：取消当前Job，执行一个新的Job
const (
	AllowConcurrent ConcurrencyPolicy = "Allow"

	ForbidConcurrent ConcurrencyPolicy = "Forbid"

	ReplaceConcurrent ConcurrencyPolicy = "Replace"
)

func init() {
	SchemeBuilder.Register(&CronJob{}, &CronJobList{})
}

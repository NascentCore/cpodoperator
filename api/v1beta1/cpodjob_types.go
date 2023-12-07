/*
Copyright 2023.

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

package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobConditionType defines all kinds of types of JobStatus.
type JobConditionType string

const (
	// JobCreated means the job has been accepted by the system,
	// but one or more of the pods/services has not been started.
	// This includes time before pods being scheduled and launched.
	JobCreated JobConditionType = "Created"

	// JobRunning means all sub-resources (e.g. services/pods) of this job
	// have been successfully scheduled and launched.
	// The training is running without error.
	JobRunning JobConditionType = "Running"

	// JobSuspended means the job is pending.
	JobPending JobConditionType = "Pending"

	// JobSucceeded means all sub-resources (e.g. services/pods) of this job
	// reached phase have terminated in success.
	// The training is complete without error.
	JobSucceeded JobConditionType = "Succeeded"

	// modelupload is in process
	ModelUploading JobConditionType = "ModelUploading"

	// modelupload is done
	ModelUploaded JobConditionType = "ModelUploaded"

	// JobFailed means one or more sub-resources (e.g. services/pods) of this job
	// reached phase failed with no restarting.
	// The training has failed its execution.
	// include the failures caused by invalid spec
	JobFailed JobConditionType = "Failed"
)

// CPodJobSpec defines the desired state of CPodJob
type CPodJobSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// GeneralJob means k8s job ,
	// +kubebuilder:validation:Enum:MPI;Pytorch;TensorFlow;GeneralJob
	JobType string `json:"jobType,omitempty"`

	// the gpu requirement for each replica
	// +kubebuilder:default:=8
	// +optional
	GPURequiredPerReplica int32 `json:"gpuRequiredPerReplica,omitempty"`
	// the path at which dataset volume will mount
	// if not set or DatasetName is not set , not dataset will be mounted ,
	// and dataset is in the image

	// gpu type will used as a nodeSelector
	GPUType string `json:"gpuType,omitempty"`

	// +optional
	DatasetPath string `json:"datasetPath,omitempty"`
	// the dataset identifier which can be mapped to a pvc volume with specified dataset
	// when DatasetName is set , this should be set
	// +optional
	DatasetName string `json:"datasetName,omitempty"`

	// the path at which pretrainmodel volume will mount
	// if not set or PretrainModelName is not set , not pretrainmodel will be mounted ,
	// and model is trained from scratch or pretrainmodel is in the image
	// +optional
	PretrainModelPath string `json:"pretrainModelPath,omitempty"`
	// the pretrainmodel identifier which can be mapped to a pvc volume with specified pretrainmodel
	// when PretrainModelPath is set , this should be set
	// +optional
	PretrainModelName string `json:"pretrainModelName,omitempty"`

	// the path at which code volume will mount
	// if not set or gitRepo is not set , not code will be mounted
	// +optional
	CodePath string `json:"codePath,omitempty"`
	// the code in git repo will be cloned to code path
	// when code path is set , this should be set
	// +optional
	GitRepo *GitRepo `json:"gitRepo,omitempty"`

	// the path at which the checkpoint volume will mount
	// +optional
	CKPTPath string `json:"ckptPath,omitempty"`
	// the size(MB) of checkpoint volume which will created by cpodoperator
	// +optional
	CKPTVolumeSize int32 `json:"ckptVolumeSize,omitempty"`

	// the path at which the modelsave volume will mount
	ModelSavePath string `json:"modelSavePath,omitempty"`
	// the size(MB) of modelsave volume which will created by cpodoperator
	ModelSaveVolumeSize int32 `json:"modelSaveVolumeSize,omitempty"`

	// total minutes for job to run , if not set or set to 0 , no time limit
	// +optional
	Duration int32 `json:"duration,omitempty"`

	// For example,
	//   {
	//     "PS": ReplicaSpec,
	//     "Worker": ReplicaSpec,
	//   }
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"replicaSpecs"`
}

// ReplicaType represents the type of the replica. Each operator needs to define its
// own set of ReplicaTypes.
type ReplicaType string

// ReplicaSpec is a description of the replica
type ReplicaSpec struct {
	// Replicas is the desired number of replicas of the given template.
	// If unspecified, defaults to 1.
	Replicas *int32 `json:"replicas,omitempty"`

	// Template is the object that describes the pod that
	// will be created for this replica. RestartPolicy in PodTemplateSpec
	// will be overide by RestartPolicy in ReplicaSpec
	Template v1.PodTemplateSpec `json:"template,omitempty"`

	// Restart policy for all replicas within the job.
	// One of Always, OnFailure, Never and ExitCode.
	// Default to Never.
	RestartPolicy RestartPolicy `json:"restartPolicy,omitempty"`
}

// RestartPolicy describes how the replicas should be restarted.
// Only one of the following restart policies may be specified.
// If none of the following policies is specified, the default one
// is RestartPolicyAlways.
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"

	// RestartPolicyExitCode policy means that user should add exit code by themselves,
	// The job operator will check these exit codes to
	// determine the behavior when an error occurs:
	// - 1-127: permanent error, do not restart.
	// - 128-255: retryable error, will restart the pod.
	RestartPolicyExitCode RestartPolicy = "ExitCode"
)

// Represents a git repository.
type GitRepo struct {
	// repository is the URL
	Repository string `json:"repository"`
	// revision is the commit hash for the specified revision.
	// +optional
	Revision string `json:"revision,omitempty"`
}

// CPodJobStatus defines the observed state of CPodJob
type CPodJobStatus struct {
	// conditions is a list of current observed job conditions.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []JobCondition `json:"conditions,omitempty"`

	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`

	// Represents time when the job was acknowledged by the job controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the job was completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Represents last time when the job was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CPodJob is the Schema for the cpodjobs API
type CPodJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CPodJobSpec   `json:"spec,omitempty"`
	Status CPodJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CPodJobList contains a list of CPodJob
type CPodJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CPodJob `json:"items"`
}

// JobCondition describes the state of the job at a certain point.
type JobCondition struct {
	// type of job condition.
	Type JobConditionType `json:"type"`

	// status of the condition, one of True, False, Unknown.
	// +kubebuilder:validation:Enum:=True;False;Unknown
	Status v1.ConditionStatus `json:"status"`

	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// A human-readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`

	// The last time this condition was updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

func init() {
	SchemeBuilder.Register(&CPodJob{}, &CPodJobList{})
}

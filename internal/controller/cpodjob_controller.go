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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cpodv1beta1 "sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/util"

	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"

	mpiv2 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	tov1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	toutil "github.com/kubeflow/training-operator/pkg/util"
)

type CPodJobOption struct {
	StorageClassName string
}

// CPodJobReconciler reconciles a CPodJob object
type CPodJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder

	Option *CPodJobOption
}

//+kubebuilder:rbac:groups=cpod.sxwl.ai,resources=cpodjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cpod.sxwl.ai,resources=cpodjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cpod.sxwl.ai,resources=cpodjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CPodJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (c *CPodJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cpodjob := &cpodv1beta1.CPodJob{}
	if err := c.Client.Get(ctx, req.NamespacedName, cpodjob); client.IgnoreNotFound(err) != nil {
		logger.Error(err, "unable to fetch CPodJob")
		return ctrl.Result{}, err
	}

	if util.IsFinshed(cpodjob.Status) {
		return ctrl.Result{}, nil
	}

	baseJob, err := c.GetBaseJob(ctx, cpodjob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = c.CreateBaseJob(ctx, *cpodjob)
			if err != nil {
				logger.Error(err, "unable to create baseJob")
				return ctrl.Result{}, err
			}
			// TODO: @sxwl-donggang update the condition of cpodjob
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	// 判断baseJob有没有到达稳定态
	baseJobStatus := c.GetBaseJobStatus(ctx, cpodjob, baseJob)
	if baseJobStatus == nil {
		logger.Info("baseJobStatus is nil")
		return ctrl.Result{Requeue: true}, nil
	}

	if err := c.UpdateStatus(ctx, cpodjob, baseJobStatus); err != nil {
		logger.Error(err, "unable to update CPodJob status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CPodJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// watch to events for cpodjob and its owned subresources basejob and pvc
	return ctrl.NewControllerManagedBy(mgr).
		For(&cpodv1beta1.CPodJob{}).
		Owns(&mpiv2.MPIJob{}).
		Complete(r)
}

func (c *CPodJobReconciler) CreateBaseJob(ctx context.Context, cpodjob cpodv1beta1.CPodJob) error {
	// 需要判断是否使用分布式训练，尽可能在节点运行，考虑以下因素
	// 1. 用户制定，需要清楚训练任务是否支持分布式训练
	// 2. 节点GPU使用数量
	// 3. 显存
	// 4. GPU型号：
	//    * 训练任务不允许使用不同信号的GPU;
	// 5. 网络：
	//     * 分布式训练任务

	var targetJob client.Object
	yes := true
	ownerRef := metav1.OwnerReference{
		APIVersion:         cpodv1beta1.GroupVersion.String(),
		Kind:               "CPodJob",
		Name:               cpodjob.Name,
		UID:                cpodjob.GetUID(),
		Controller:         &yes,
		BlockOwnerDeletion: &yes,
	}

	// 处理挂卷逻辑
	// TODO: @sxwl-donggang为什么需要挂载这么多卷？
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	if cpodjob.Spec.CKPTPath != "" {
		ckptPVC, err := c.GetCKPTPVC(ctx, &cpodjob)
		if err != nil {
			c.Recorder.Eventf(&cpodjob, corev1.EventTypeWarning, "GetCKPTPVCFailed", "Get ckpt pvc failed")
			return err
		}
		volumes = append(volumes, corev1.Volume{
			Name: "ckpt",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: ckptPVC.Name,
					ReadOnly:  false,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "ckpt",
			MountPath: cpodjob.Spec.CKPTPath,
		})
	}
	// TODO: @sxwl-donggang 挂载其他卷

	runpolicy := tov1.RunPolicy{
		CleanPodPolicy: tov1.CleanPodPolicyPointer(tov1.CleanPodPolicyRunning),
		// TODO: @sxwl-donggang
		// SchedulingPolicy:
	}

	switch cpodjob.Spec.JobType {
	case cpodv1beta1.JobTypeMPI:
		targetJob = &mpiv2.MPIJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpodjob.Name,
				Namespace: cpodjob.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					ownerRef,
				},
			},
			Spec: mpiv2.MPIJobSpec{
				MPIReplicaSpecs: map[mpiv2.MPIReplicaType]*commonv1.ReplicaSpec{
					mpiv2.MPIReplicaTypeLauncher: {
						RestartPolicy: commonv1.RestartPolicyOnFailure,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Env:          cpodjob.Spec.Envs,
										Name:         "launcher",
										Command:      cpodjob.Spec.Command,
										Image:        cpodjob.Spec.Image,
										VolumeMounts: volumeMounts,
									},
								},
								HostIPC: true,
								Volumes: volumes,
							},
						},
					},
					mpiv2.MPIReplicaTypeWorker: {
						Replicas:      &cpodjob.Spec.WorkerReplicas,
						RestartPolicy: commonv1.RestartPolicyOnFailure,
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Env:          cpodjob.Spec.Envs,
										Image:        cpodjob.Spec.Image,
										VolumeMounts: volumeMounts,
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
											},
										},
									},
								},
								Volumes: volumes,
								NodeSelector: map[string]string{
									"nvidia.com/gpu.product": cpodjob.Spec.GPUType,
								},
							},
						},
					},
				},

				// RunPolicy: mpiv2.RunPolicy(runpolicy),
			},
		}
	case cpodv1beta1.JobTypePytorch:
		backendC10D := tov1.BackendC10D
		targetJob = &tov1.PyTorchJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpodjob.Name,
				Namespace: cpodjob.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					ownerRef,
				},
			},
			Spec: tov1.PyTorchJobSpec{
				ElasticPolicy: &tov1.ElasticPolicy{
					RDZVBackend: &backendC10D,
				},
				RunPolicy: runpolicy,
				PyTorchReplicaSpecs: map[tov1.ReplicaType]*tov1.ReplicaSpec{
					// 不设置Master
					tov1.PyTorchJobReplicaTypeWorker: {
						Replicas:      &cpodjob.Spec.WorkerReplicas,
						RestartPolicy: tov1.RestartPolicyOnFailure,
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Env:          cpodjob.Spec.Envs,
										Image:        cpodjob.Spec.Image,
										Command:      cpodjob.Spec.Command,
										VolumeMounts: volumeMounts,
										Resources: corev1.ResourceRequirements{
											Requests: corev1.ResourceList{
												corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
											},
										},
									},
								},
								Volumes: volumes,
								NodeSelector: map[string]string{
									"nvidia.com/gpu.product": cpodjob.Spec.GPUType,
								},
							},
						},
					},
				},
			},
		}
	}
	return client.IgnoreAlreadyExists(c.Client.Create(ctx, targetJob))
}

func (c *CPodJobReconciler) UpdateStatus(ctx context.Context, cpodjob *cpodv1beta1.CPodJob, baseJobStatus *tov1.JobStatus) error {
	if cpodjob.Status.StartTime == nil {
		now := metav1.Now()
		cpodjob.Status.StartTime = &now
	}

	if toutil.IsFailed(*baseJobStatus) {
		baseJobFailedCond := util.GetBaseJobCondition(*baseJobStatus, tov1.JobFailed)
		util.UpdateJobConditions(&cpodjob.Status, cpodv1beta1.JobFailed, corev1.ConditionTrue, "BaseJobFailed", baseJobFailedCond.Message)
	} else if toutil.IsSucceeded(*baseJobStatus) {
		baseJobSucceedCond := util.GetBaseJobCondition(*baseJobStatus, tov1.JobSucceeded)
		util.UpdateJobConditions(&cpodjob.Status, cpodv1beta1.JobSucceeded, corev1.ConditionTrue, baseJobSucceedCond.Reason, baseJobSucceedCond.Message)
		cpodjob.Status.CompletionTime = baseJobStatus.CompletionTime
	} else {
		util.UpdateJobConditions(&cpodjob.Status, cpodv1beta1.JobRunning, corev1.ConditionTrue, "BaseJobRunning", "BaseJob is running")
	}

	return c.Status().Update(ctx, cpodjob)
}

// GetBaseJob retrieves the base job object based on the job type specified in the CPodJob.
// It returns the target job object and an error, if any.
func (c *CPodJobReconciler) GetBaseJob(ctx context.Context, cpodjob *cpodv1beta1.CPodJob) (client.Object, error) {
	var targetJob client.Object
	switch cpodjob.Spec.JobType {
	case cpodv1beta1.JobTypeMPI:
		targetJob = &mpiv2.MPIJob{}
	case cpodv1beta1.JobTypePytorch:
		targetJob = &tov1.PyTorchJob{}
	}
	return targetJob, c.Get(ctx, client.ObjectKey{Namespace: cpodjob.Namespace, Name: cpodjob.Name}, targetJob)
}

// 由于MPIJob使用的是mpi-controller中的定义，与training-operator的定义不一致，需要进行转换
func (c *CPodJobReconciler) GetBaseJobStatus(ctx context.Context, cpodjob *cpodv1beta1.CPodJob, baseJob client.Object) *tov1.JobStatus {
	switch cpodjob.Spec.JobType {
	case cpodv1beta1.JobTypePytorch:
		pytJob := baseJob.(*tov1.PyTorchJob)
		return &pytJob.Status
	case cpodv1beta1.JobTypeMPI:
		mpiJob := baseJob.(*mpiv2.MPIJob)
		return &tov1.JobStatus{
			Conditions: c.ConvertMPIJobConditionToCommonJobCondition(ctx, mpiJob.Status.Conditions),
			// TODO: @sxwl-donggang ReplicaStatuses is currently not used, consider if it needs to be used in the future
			StartTime:         mpiJob.Status.StartTime,
			LastReconcileTime: mpiJob.Status.LastReconcileTime,
			CompletionTime:    mpiJob.Status.CompletionTime,
		}
	default:
		return nil
	}
}

// 将mpiv1.JobCondition转换为commonv1.JobCondition
func (c *CPodJobReconciler) ConvertMPIJobConditionToCommonJobCondition(ctx context.Context, mpiJobConditions []mpiv2.JobCondition) []tov1.JobCondition {
	res := make([]tov1.JobCondition, len(mpiJobConditions))
	for i, mpiJobCondition := range mpiJobConditions {
		res[i] = tov1.JobCondition{
			Type:               tov1.JobConditionType(mpiJobCondition.Type),
			Status:             mpiJobCondition.Status,
			LastUpdateTime:     mpiJobCondition.LastUpdateTime,
			LastTransitionTime: mpiJobCondition.LastTransitionTime,
			Reason:             mpiJobCondition.Reason,
			Message:            mpiJobCondition.Message,
		}
	}
	return res
}

func (c *CPodJobReconciler) GetCKPTPVC(ctx context.Context, cpodjob *cpodv1beta1.CPodJob) (*corev1.PersistentVolumeClaim, error) {
	logger := log.FromContext(ctx)
	ckptPVCName := cpodjob.Name + "-ckpt"
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: cpodjob.Namespace, Name: ckptPVCName}, pvc); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ckpt pvc not found, create it")
			yes := true
			volumeMode := corev1.PersistentVolumeFilesystem
			err := c.Client.Create(ctx, &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ckptPVCName,
					Namespace: cpodjob.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         cpodv1beta1.GroupVersion.String(),
							Kind:               "CPodJob",
							Name:               cpodjob.Name,
							UID:                cpodjob.GetUID(),
							Controller:         &yes,
							BlockOwnerDeletion: &yes,
						},
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: *resource.NewQuantity(int64(cpodjob.Spec.CKPTVolumeSize), resource.BinarySI),
						},
					},
					StorageClassName: &c.Option.StorageClassName,
					VolumeMode:       &volumeMode,
				},
			})
			if err != nil {
				logger.Error(err, "create ckpt pvc failed")
				return nil, err
			}
		}
		return nil, err
	}

	// Check if the PVC is bound
	if pvc.Status.Phase != corev1.ClaimBound {
		logger.Info("ckpt pvc not bound, wait for binding")
		return nil, fmt.Errorf("ckpt pvc not bound, wait for binding")
	}
	return pvc, nil
}

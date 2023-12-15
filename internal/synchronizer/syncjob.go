package synchronizer

import (
	"context"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/provider/sxwl"

	"github.com/go-logr/logr"
	tov1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SyncJob struct {
	kubeClient client.Client
	scheduler  sxwl.Scheduler
	logger     logr.Logger
}

func NewSyncJob(kubeClient client.Client, scheduler sxwl.Scheduler, logger logr.Logger) *SyncJob {
	return &SyncJob{
		kubeClient: kubeClient,
		scheduler:  scheduler,
		logger:     logger,
	}
}

func (s *SyncJob) Start(ctx context.Context) {
	s.logger.Info("sync task")

	var cpodjobs v1beta1.CPodJobList
	err := s.kubeClient.List(ctx, &cpodjobs, &client.MatchingLabels{
		v1beta1.CPodJobSourceLabel: v1beta1.CPodJobSource,
	})
	if err != nil {
		s.logger.Error(err, "failed to list cpodjob")
		return
	}

	tasks, err := s.scheduler.GetAssignedJobList()
	if err != nil {
		s.logger.Error(err, "failed to list task")
		return
	}
	s.logger.Info("assigned task", "tasks", tasks)

	for _, task := range tasks {
		exists := false
		for _, cpodjob := range cpodjobs.Items {
			if cpodjob.Name == task.JobName {
				exists = true
			}
		}
		if !exists {
			newCPodJob := v1beta1.CPodJob{
				ObjectMeta: metav1.ObjectMeta{
					// TODO: create namespace for different tenant
					Namespace: "cpod",
					Name:      task.JobName,
				},
				Spec: v1beta1.CPodJobSpec{
					JobType:             v1beta1.JobType(task.JobType),
					GPUType:             task.GpuType,
					DatasetName:         task.DatasetName,
					DatasetPath:         task.DatasetPath,
					PretrainModelName:   task.PretrainModelName,
					PretrainModelPath:   task.PretrainModelPath,
					CKPTPath:            task.CkptPath,
					CKPTVolumeSize:      int32(task.CkptVol),
					ModelSavePath:       task.ModelPath,
					ModelSaveVolumeSize: int32(task.ModelVol),
					ReplicaSpecs: map[tov1.ReplicaType]*tov1.ReplicaSpec{
						// TODO: @sxwl-donggang 如何制定type
						tov1.PyTorchJobReplicaTypeWorker: {
							Template: v1.PodTemplateSpec{
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:    "main",
											Image:   task.ImagePath,
											Command: []string{task.Command},
										},
									},
								},
							},
						},
					},
				},
			}
			if err = s.kubeClient.Create(ctx, &newCPodJob); err != nil {
				s.logger.Error(err, "failed to create cpodjob")
				return
			}
		}
	}

	for _, cpodjob := range cpodjobs.Items {
		exists := false
		for _, task := range tasks {
			if cpodjob.Name == task.JobName {
				exists = true
			}
		}
		if !exists {
			if err = s.kubeClient.Delete(ctx, &cpodjob); err != nil {
				s.logger.Error(err, "failed to delete cpodjob")
				return
			}
		}
	}

}

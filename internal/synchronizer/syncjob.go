package synchronizer

import (
	"context"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/provider/sxwl"
	"sync"

	"github.com/go-logr/logr"
	tov1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type jobBuffer struct {
	m  map[string]sxwl.PortalJob
	mu *sync.RWMutex
}

type SyncJob struct {
	kubeClient       client.Client
	scheduler        sxwl.Scheduler
	createFailedJobs jobBuffer
	logger           logr.Logger
}

func NewSyncJob(kubeClient client.Client, scheduler sxwl.Scheduler, logger logr.Logger) *SyncJob {
	return &SyncJob{
		kubeClient:       kubeClient,
		scheduler:        scheduler,
		createFailedJobs: jobBuffer{m: map[string]sxwl.PortalJob{}, mu: new(sync.RWMutex)},
		logger:           logger,
	}
}

// first retrieve twn job sets , portal job set and cpod job set
// for jobs in portal not in cpod , create it
// for jobs in cpod not in portal , if it's running , delete it
func (s *SyncJob) Start(ctx context.Context) {
	s.logger.Info("sync job")

	var cpodjobs v1beta1.CPodJobList
	err := s.kubeClient.List(ctx, &cpodjobs, &client.MatchingLabels{
		v1beta1.CPodJobSourceLabel: v1beta1.CPodJobSource,
	})
	if err != nil {
		s.logger.Error(err, "failed to list cpodjob")
		return
	}

	portaljobs, err := s.scheduler.GetAssignedJobList()
	if err != nil {
		s.logger.Error(err, "failed to list job")
		return
	}
	s.logger.Info("assigned job", "jobs", portaljobs)

	for _, job := range portaljobs {
		exists := false
		for _, cpodjob := range cpodjobs.Items {
			if cpodjob.Name == job.JobName {
				exists = true
			}
		}
		if !exists {
			newCPodJob := v1beta1.CPodJob{
				ObjectMeta: metav1.ObjectMeta{
					// TODO: create namespace for different tenant
					Namespace: "cpod",
					Name:      job.JobName,
				},
				Spec: v1beta1.CPodJobSpec{
					JobType:             v1beta1.JobType(job.JobType),
					GPUType:             job.GpuType,
					DatasetName:         job.DatasetName,
					DatasetPath:         job.DatasetPath,
					PretrainModelName:   job.PretrainModelName,
					PretrainModelPath:   job.PretrainModelPath,
					CKPTPath:            job.CkptPath,
					CKPTVolumeSize:      int32(job.CkptVol),
					ModelSavePath:       job.ModelPath,
					ModelSaveVolumeSize: int32(job.ModelVol),
					ReplicaSpecs: map[tov1.ReplicaType]*tov1.ReplicaSpec{
						// TODO: @sxwl-donggang 如何制定type
						tov1.PyTorchJobReplicaTypeWorker: {
							Template: v1.PodTemplateSpec{
								Spec: v1.PodSpec{
									Containers: []v1.Container{
										{
											Name:    "main",
											Image:   job.ImagePath,
											Command: []string{job.Command},
										},
									},
								},
							},
						},
					},
				},
			}
			if err = s.kubeClient.Create(ctx, &newCPodJob); err != nil {
				s.addCreateFailedJob(job)
				s.logger.Error(err, "failed to create cpodjob")
			} else {
				s.deleteCreateFailedJob(job.JobName)
			}
		}
	}

	for _, cpodjob := range cpodjobs.Items {
		// TODO: add NoMoreChange in api , use NoMoreChange instead of != v1beta1.CPodJobRunning
		// do nothing if job has reached a no more change status
		if cpodjob.Status.Phase != v1beta1.CPodJobRunning {
			continue
		}
		exists := false
		for _, job := range portaljobs {
			if cpodjob.Name == job.JobName {
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

func (s *SyncJob) getCreateFailedJobs() []sxwl.PortalJob {
	res := []sxwl.PortalJob{}
	s.createFailedJobs.mu.RLock()
	defer s.createFailedJobs.mu.RUnlock()
	for _, v := range s.createFailedJobs.m {
		res = append(res, v)
	}
	return res
}

func (s *SyncJob) addCreateFailedJob(j sxwl.PortalJob) {
	if _, ok := s.createFailedJobs.m[j.JobName]; ok {
		return
	}
	s.createFailedJobs.mu.Lock()
	defer s.createFailedJobs.mu.Unlock()
	s.createFailedJobs.m[j.JobName] = j
}

// 如果任务创建成功了，将其从失败任务列表中删除
func (s *SyncJob) deleteCreateFailedJob(j string) {
	if _, ok := s.createFailedJobs.m[j]; !ok {
		return
	}
	s.createFailedJobs.mu.Lock()
	defer s.createFailedJobs.mu.Unlock()
	delete(s.createFailedJobs.m, j)
}

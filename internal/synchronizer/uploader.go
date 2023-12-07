package synchronizer

import (
	"context"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/provider/sxwl"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Uploader定时上报任务
type Uploader struct {
	kubeClient client.Client
	scheduler  sxwl.Scheduler
}

func NewUploader(kubeClient client.Client, scheduler sxwl.Scheduler) *Uploader {
	return &Uploader{
		kubeClient: kubeClient,
		scheduler:  scheduler,
	}
}

func (u *Uploader) Run(ctx context.Context) {
	// fetch all cpojob
	logger := log.FromContext(ctx).WithName("uploader")

	var cpodjobs v1beta1.CPodJobList
	err := u.kubeClient.List(ctx, &cpodjobs, &client.MatchingLabels{
		v1beta1.CPodJobSourceLabel: v1beta1.CPodJobSource,
	})
	if err != nil {
		logger.Error(err, "failed to list cpodjob")
		return
	}

	stats := make([]sxwl.State, len(cpodjobs.Items))
	for _, cpod := range cpodjobs.Items {
		stats = append(stats, sxwl.State{
			Name:      cpod.Name,
			Namespace: cpod.Namespace,
			JobType:   cpod.Spec.JobType,
			JobStatus: v1beta1.CPodJobPhase(cpod.Status.Phase),
		})
	}

	err = u.scheduler.TaskCallBack(stats)
	if err != nil {
		logger.Error(err, "failed to list cpodjob")
		return
	}
	logger.Info("uploader", "Stats", stats)
}

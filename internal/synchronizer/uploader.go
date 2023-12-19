package synchronizer

import (
	"context"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/provider/sxwl"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Uploader定时上报任务
type Uploader struct {
	kubeClient client.Client
	scheduler  sxwl.Scheduler
	logger     logr.Logger
}

func NewUploader(kubeClient client.Client, scheduler sxwl.Scheduler, logger logr.Logger) *Uploader {
	return &Uploader{
		kubeClient: kubeClient,
		scheduler:  scheduler,
		logger:     logger,
	}
}

func (u *Uploader) Start(ctx context.Context) {
	// fetch all cpojob
	u.logger.Info("uploader")

	var cpodjobs v1beta1.CPodJobList
	err := u.kubeClient.List(ctx, &cpodjobs, &client.MatchingLabels{
		v1beta1.CPodJobSourceLabel: v1beta1.CPodJobSource,
	})
	if err != nil {
		u.logger.Error(err, "failed to list cpodjob")
		return
	}

	stats := make([]sxwl.State, len(cpodjobs.Items))
	for _, cpod := range cpodjobs.Items {
		stats = append(stats, sxwl.State{
			Name:      cpod.Name,
			Namespace: cpod.Namespace,
			JobType:   cpod.Spec.JobType,
			// JobStatus: v1beta1.CPodJobPhase(cpod.Status.Phase),
		})
		err = u.scheduler.TaskCallBack(stats)
		if err != nil {
			u.logger.Error(err, "failed to list cpodjob")
			return
		}
		u.logger.Info("uploader", "Stats", stats)
	}
}

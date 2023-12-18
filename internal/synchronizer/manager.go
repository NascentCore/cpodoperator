package synchronizer

import (
	"context"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/provider/sxwl"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Runnable interface {
	Start(ctx context.Context)
}

type Manager struct {
	// runnables is a collection of all the runnables that are managed by this manager.
	runables []Runnable

	logger logr.Logger

	period time.Duration
}

func NewManager(cpodId string, kubeClient client.Client, scheduler sxwl.Scheduler, period time.Duration, logger logr.Logger) *Manager {
	ch := make(chan sxwl.HeartBeatPayload, 1)
	syncJob := NewSyncJob(kubeClient, scheduler, logger.WithName("syncjob"))
	uploader := NewUploader(ch, scheduler, period, logger.WithName("uploader"))
	cpodObserver := NewCPodObserver(kubeClient, cpodId, v1beta1.CPOD_NAMESPACE, ch, syncJob.getCreateFailedJobs, logger.WithName("cpodobserver"))
	return &Manager{
		runables: []Runnable{
			syncJob,
			uploader,
			cpodObserver,
		},
		period: period,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	for _, runable := range m.runables {
		runable := runable
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			runable.Start(ctx)
		}, m.period)
	}
	return nil
}

package synchronizer

import (
	"context"
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

func NewManager(kubeClient client.Client, scheduler sxwl.Scheduler, period time.Duration, logger logr.Logger) *Manager {
	return &Manager{
		runables: []Runnable{
			NewSyncJob(kubeClient, scheduler, logger.WithName("syncjob")),
			NewUploader(kubeClient, scheduler, logger.WithName("uploader")),
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

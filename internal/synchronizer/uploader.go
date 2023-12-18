package synchronizer

import (
	"context"
	"sxwl/cpodoperator/pkg/provider/sxwl"
	"time"

	"github.com/go-logr/logr"
)

// Uploader定时上报心跳信息, 数据来自Ch
type Uploader struct {
	ch        <-chan sxwl.HeartBeatPayload
	scheduler sxwl.Scheduler
	interval  time.Duration
	logger    logr.Logger
}

func NewUploader(ch <-chan sxwl.HeartBeatPayload, scheduler sxwl.Scheduler, interval time.Duration, logger logr.Logger) *Uploader {
	return &Uploader{
		ch:        ch,
		scheduler: scheduler,
		interval:  interval,
		logger:    logger,
	}
}

func (u *Uploader) Start(ctx context.Context) {
	// fetch all cpojob
	u.logger.Info("uploader")
	// upload data , even data is not updated
	var payload sxwl.HeartBeatPayload
	select {
	case payload = <-u.ch:
	case <-ctx.Done():
		u.logger.Info("uploader stopped")
		return
	}
	for {
		time.Sleep(u.interval)
		select {
		case payload = <-u.ch:
		case <-ctx.Done():
			u.logger.Info("uploader stopped")
			return
		default:
			// do nothing ,  still do the upload but data will be old
			u.logger.Info("cpod status data is not refreshed , upload the old data")
		}
		err := u.scheduler.HeartBeat(payload)
		if err != nil {
			u.logger.Error(err, "upload cpod status data failed")
		} else {
			u.logger.Info("uploaded cpod status data")
		}
	}

}

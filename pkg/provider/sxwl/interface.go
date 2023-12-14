package sxwl

import (
	"net/http"
	"sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/pkg/resource"
	"time"
)

type PortalJob struct {
	CkptPath    string            `json:"ckptPath"`
	CkptVol     int               `json:"ckptVol"`
	Command     string            `json:"runCommand"`
	Envs        map[string]string `json:"env"`
	DatasetPath string            `json:"datasetPath"`
	DatasetName string            `json:"DatasetName"`
	GpuNumber   int               `json:"gpuNumber"`
	GpuType     string            `json:"gpuType"`
	HfURL       string            `json:"hfUrl"`
	// TODO: @sxwl-donggang rename to Image
	ImagePath         string `json:"imagePath"`
	JobID             int    `json:"jobId"`
	JobName           string `json:"jobName"`
	JobType           string `json:"jobType"`
	ModelPath         string `json:"modelPath"`
	ModelVol          int    `json:"modelVol"`
	PretrainModelName string `json:"pretrainedModelName"`
	PretrainModelPath string `json:"pretrainedModelPath"`
	StopTime          int    `json:"stopTime"`
	StopType          int    `json:"stopType"`
}

type State struct {
	Name      string          `json:"name"`
	Namespace string          `json:"namespace"`
	JobType   v1beta1.JobType `json:"jobtype"`
	// TODO: @sxwl-donggang 序列化风格没保持一致，第一版竟然让sxwl不变更
	JobStatus v1beta1.CPodJobPhase `json:"job_status"`
	Info      string               `json:"info,omitempty"` // more info about jobstatus , especially when error occured
	Extension interface{}          `json:"extension"`
}

type Scheduler interface {
	// GetAssignedJobList get assigned to this  Job  from scheduler
	GetAssignedJobList() ([]PortalJob, error)

	// upload heartbeat info ,
	HeartBeat(HeartBeatPayload) error
}

type HeartBeatPayload struct {
	CPodID       string                    `json:"cpod_id"`
	JobStatus    []State                   `json:"job_status"`
	ResourceInfo resource.CPodResourceInfo `json:"resource_info"`
	UpdateTime   time.Time                 `json:"update_time"`
}

func NewScheduler(baseURL, accesskey, identify string) Scheduler {
	return &sxwl{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		baseURL:   baseURL,
		accessKey: accesskey,
		identity:  identify,
	}
}

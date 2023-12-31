package synchronizer

import (
	"context"
	"strconv"
	"strings"
	"sxwl/cpodoperator/api/v1beta1"

	cpodv1 "sxwl/cpodoperator/api/v1"
	"sxwl/cpodoperator/pkg/provider/sxwl"
	"sxwl/cpodoperator/pkg/resource"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CPodObserver struct {
	kubeClient             client.Client
	logger                 logr.Logger
	ch                     chan<- sxwl.HeartBeatPayload
	createFailedJobsGetter func() []sxwl.PortalJob
	cpodId                 string
	cpodNamespace          string
}

func NewCPodObserver(kubeClient client.Client, cpodId, cpodNamespace string, ch chan<- sxwl.HeartBeatPayload,
	createFailedJobsGetter func() []sxwl.PortalJob, logger logr.Logger) *CPodObserver {
	return &CPodObserver{kubeClient: kubeClient, logger: logger, ch: ch, createFailedJobsGetter: createFailedJobsGetter,
		cpodId: cpodId, cpodNamespace: cpodNamespace}
}

func (co *CPodObserver) Start(ctx context.Context) {
	co.logger.Info("cpod observer")
	js, err := co.getJobStates(ctx)
	if err != nil {
		co.logger.Error(err, "get job state error")
		return
	}
	// combine with createdfailed jobs
	for _, j := range co.createFailedJobsGetter() {
		js = append(js, sxwl.State{
			Name:      j.JobName,
			Namespace: co.cpodNamespace,
			JobType:   v1beta1.JobType(j.JobType),
			// TODO: add to api or const
			JobStatus: "createfailed",
		})
	}
	co.logger.Info("jobstates to upload", "js", js)
	resourceInfo, err := co.getResourceInfo(ctx)
	if err != nil {
		co.logger.Error(err, "get resource error")
		return
	}
	co.ch <- sxwl.HeartBeatPayload{
		CPodID:       co.cpodId,
		ResourceInfo: resourceInfo,
		JobStatus:    js,
		UpdateTime:   time.Now(),
	}
	co.logger.Info("upload payload refreshed")
}

func parseStatus(s v1beta1.CPodJobStatus) (v1beta1.JobConditionType, string) {
	conditions := s.Conditions
	if len(conditions) == 0 {
		return v1beta1.JobCreated, "no status"
	}
	condMap := map[v1beta1.JobConditionType]v1beta1.JobCondition{}
	for _, cond := range conditions {
		condMap[cond.Type] = cond
	}
	// order : modeluploaded , modeluploading , failed , succeeded , running , created
	checkOrder := []v1beta1.JobConditionType{v1beta1.JobModelUploaded, v1beta1.JobModelUploading, v1beta1.JobFailed,
		v1beta1.JobSucceeded, v1beta1.JobRunning, v1beta1.JobCreated}
	for _, checker := range checkOrder {
		if condMap[checker].Status == v1.ConditionTrue {
			return checker, condMap[checker].Message
		}
	}
	return v1beta1.JobCreated, "unknow status"
}

func (co *CPodObserver) getJobStates(ctx context.Context) ([]sxwl.State, error) {
	var cpodjobs v1beta1.CPodJobList
	err := co.kubeClient.List(ctx, &cpodjobs, &client.MatchingLabels{
		v1beta1.CPodJobSourceLabel: v1beta1.CPodJobSource,
	})
	if err != nil {
		return nil, err
	}

	stats := []sxwl.State{}
	for _, cpodjob := range cpodjobs.Items {
		status, info := parseStatus(cpodjob.Status)
		// it'a time limit job
		if cpodjob.Spec.Duration > 0 {
			createTime := cpodjob.CreationTimestamp.Time
			if createTime.Add(time.Duration(cpodjob.Spec.Duration) * time.Minute).Before(time.Now()) {
				// TODO: add new status  TimeUp
				if status == v1beta1.JobRunning {
					info = "timeup when running"
					status = v1beta1.JobSucceeded
				} else {
					info = "timeup job not running"
					status = v1beta1.JobFailed
				}
			}
		}

		stats = append(stats, sxwl.State{
			Name:      cpodjob.Name,
			Namespace: cpodjob.Namespace,
			JobType:   cpodjob.Spec.JobType,
			// TODO: synch defination with portal
			JobStatus: v1beta1.JobConditionType(strings.ToLower(string(status))),
			Info:      info,
		})
	}
	return stats, nil
}

func (co *CPodObserver) getResourceInfo(ctx context.Context) (resource.CPodResourceInfo, error) {
	var info resource.CPodResourceInfo
	info.CPodID = co.cpodId
	info.CPodVersion = "v1.0"
	// get node list from k8s
	info.Nodes = []resource.NodeInfo{}
	var nodeInfo corev1.NodeList
	err := co.kubeClient.List(ctx, &nodeInfo)
	if err != nil {
		return info, err
	}
	for _, node := range nodeInfo.Items {
		t := resource.NodeInfo{}
		t.Name = node.Name
		t.Status = node.Labels["status"]
		t.KernelVersion = node.Labels["feature.node.kubernetes.io/kernel-version.full"]
		t.LinuxDist = node.Status.NodeInfo.OSImage
		t.Arch = node.Labels["kubernetes.io/arch"]
		t.CPUInfo.Cores = int(node.Status.Capacity.Cpu().Value())
		if v, ok := node.Labels[v1beta1.K8S_LABEL_NV_GPU_PRODUCT]; ok {
			t.GPUInfo.Prod = v
			t.GPUInfo.Vendor = "nvidia"
			t.GPUInfo.Driver = node.Labels["nvidia.com/cuda.driver.major"] + "." +
				node.Labels["nvidia.com/cuda.driver.minor"] + "." +
				node.Labels["nvidia.com/cuda.driver.rev"]
			t.GPUInfo.CUDA = node.Labels["nvidia.com/cuda.runtime.major"] + "." +
				node.Labels["nvidia.com/cuda.runtime.minor"]
			t.GPUInfo.MemSize, _ = strconv.Atoi(node.Labels["nvidia.com/gpu.memory"])
			t.GPUInfo.Status = "abnormal"
			if node.Labels[v1beta1.K8S_LABEL_NV_GPU_PRESENT] == "true" {
				t.GPUInfo.Status = "normal"
			}
			//init GPUState Array , accordding to nvidia.com/gpu.count label
			t.GPUState = []resource.GPUState{}
			gpuCnt, _ := strconv.Atoi(node.Labels["nvidia.com/gpu.count"])
			t.GPUTotal = gpuCnt
			for i := 0; i < gpuCnt; i++ {
				t.GPUState = append(t.GPUState, resource.GPUState{})
			}
			tmp := node.Status.Allocatable["nvidia.com/gpu"].DeepCopy()
			if i, ok := (&tmp).AsInt64(); ok {
				t.GPUAllocatable = int(i)
			}
		}
		t.MemInfo.Size = int(node.Status.Capacity.Memory().Value() / 1024 / 1024)
		info.Nodes = append(info.Nodes, t)
	}
	//stat gpus in cpod
	statTotal := map[[2]string]int{}
	statAlloc := map[[2]string]int{}
	for _, node := range info.Nodes {
		statTotal[[2]string{node.GPUInfo.Vendor, node.GPUInfo.Prod}] += node.GPUTotal
		statAlloc[[2]string{node.GPUInfo.Vendor, node.GPUInfo.Prod}] += node.GPUAllocatable
	}
	for k, v := range statTotal {
		info.GPUSummaries = append(info.GPUSummaries, resource.GPUSummary{
			Vendor:      k[0],
			Prod:        k[1],
			Total:       v,
			Allocatable: statAlloc[k],
		})
	}

	ms, ds, err := co.getExistingArtifacts(ctx)
	if err != nil {
		return resource.CPodResourceInfo{}, err
	}
	info.CachedModels = ms
	info.CachedDatasets = ds
	images, err := co.getImages(ctx)
	if err != nil {
		return resource.CPodResourceInfo{}, err
	}
	info.CachedImages = images
	return info, nil
}

// return dataset list \ model list \ err
func (co *CPodObserver) getExistingArtifacts(ctx context.Context) ([]string, []string, error) {
	var modelList cpodv1.ModelStorageList
	err := co.kubeClient.List(ctx, &modelList)
	if err != nil {
		return nil, nil, err
	}
	models := []string{}
	for _, model := range modelList.Items {
		models = append(models, model.Spec.ModelName)
	}
	var datasetList cpodv1.DataSetStorageList
	err = co.kubeClient.List(ctx, &datasetList)
	if err != nil {
		return nil, nil, err
	}
	datasets := []string{}
	for _, dataset := range datasetList.Items {
		datasets = append(datasets, dataset.Spec.DatasetName)
	}
	return models, datasets, nil
}

// TODO: read image list from harbor
func (co *CPodObserver) getImages(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

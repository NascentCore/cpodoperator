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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	cpodv1beta1 "sxwl/cpodoperator/api/v1beta1"
	"sxwl/cpodoperator/internal/controller"
	"sxwl/cpodoperator/internal/synchronizer"
	"sxwl/cpodoperator/pkg/provider/sxwl"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(cpodv1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var downloaderImage string
	var storageClassName string
	var syncPeriod int
	var sxwlBaseUrl string
	var sxwlAccessKey string
	var sxwlIdentity string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&downloaderImage, "artifact-downloader-image", "sxwl-registry.cn-beijing.cr.aliyuncs.com/sxwl-ai/downloader:v1.0.0", "The artifact download job image ")
	flag.StringVar(&storageClassName, "storageClassName", "ceph-filesystem", "which storagecalss the artifact downloader should create")
	flag.IntVar(&syncPeriod, "sync-period", 10, "the period of every run of synchronizer, unit is second")
	flag.StringVar(&sxwlBaseUrl, "sxwl-baseurl", "https://aiapi.yangapi.cn", "the sxwl url ")
	flag.StringVar(&sxwlAccessKey, "sxwl-accesskey", "", "the access key to access sxwl ")
	flag.StringVar(&sxwlIdentity, "sxwl-identity", "", "the identity to access sxwl ")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "2ca7307e.sxwl.ai",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.CPodJobReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CPodJob")
		os.Exit(1)
	}
	// if err = (&controller.ModelStorageReconciler{
	// 	Client: mgr.GetClient(),
	// 	Scheme: mgr.GetScheme(),
	// 	Option: &controller.ModelStorageOption{
	// 		DownloaderImage:  downloaderImage,
	// 		StorageClassName: storageClassName,
	// 	},
	// }).SetupWithManager(mgr); err != nil {
	// 	setupLog.Error(err, "unable to create controller", "controller", "ModelStorage")
	// 	os.Exit(1)
	// }
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	// TODO: @sxwl-donggang 自定Ready checker
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	accessKey := os.Getenv("ACCESS_KEY") //from configmap provided by cairong
	cpodId := os.Getenv("CPOD_ID")       //from configmap provided by cairong
	syncManager := synchronizer.NewManager(cpodId, mgr.GetClient(), sxwl.NewScheduler(sxwlBaseUrl, accessKey, cpodId), time.Duration(syncPeriod)*time.Second, ctrl.Log)
	go func() {
		if mgr.GetCache().WaitForCacheSync(ctx) {
			syncManager.Start(ctx)
		} else {
			setupLog.Error(fmt.Errorf("cannot wait for cache sync"), "problem waiting informer cache")
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

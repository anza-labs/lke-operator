/*
Copyright 2024 lke-operator contributors.

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
	"crypto/tls"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/anza-labs/lke-operator/internal/controller"
	tracedk8s "github.com/anza-labs/lke-operator/internal/k8s/traced"
	"github.com/anza-labs/lke-operator/internal/version"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	lkev1alpha1 "github.com/anza-labs/lke-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(lkev1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		secureMetrics        bool
		enableHTTP2          bool
	)

	flag.StringVar(
		&metricsAddr,
		"metrics-bind-address",
		":8080",
		"The address the metric endpoint binds to.",
	)

	flag.StringVar(
		&probeAddr,
		"health-probe-bind-address",
		":8081",
		"The address the probe endpoint binds to.",
	)

	flag.BoolVar(
		&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)

	flag.BoolVar(
		&secureMetrics,
		"metrics-secure",
		false,
		"If set the metrics endpoint is served securely.",
	)

	flag.BoolVar(
		&enableHTTP2,
		"enable-http2",
		false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers.",
	)

	klog.InitFlags(nil)
	flag.Parse()
	ctrl.SetLogger(klog.Background())

	setupLog.Info("starting manager",
		"version", version.Version)

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:                 webhookServer,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "7006d143.anza-labs.dev",
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.LKEClusterConfigReconciler{
		Client:           tracedk8s.NewClientWithTracing(mgr.GetClient(), "main_mgr_client"),
		Scheme:           mgr.GetScheme(),
		KubernetesClient: kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie()),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller",
			"controller", "LKEClusterConfig")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

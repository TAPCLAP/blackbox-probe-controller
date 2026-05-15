/*
Copyright 2026.

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
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/TAPCLAP/blackbox-probe-controller/internal/cluster"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/controller"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/options"
	"github.com/TAPCLAP/blackbox-probe-controller/internal/state"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(vmv1beta1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)

	var vmprobeNamespace string
	var clusterSecretNamespace string
	var probeInterval string
	var probeScrapeTimeout string
	var probeModule string
	var blackboxProberURL string

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	flag.StringVar(&vmprobeNamespace, "vmprobe-namespace", "monitoring", "Namespace for managed VMProbe resources")
	flag.StringVar(&clusterSecretNamespace, "cluster-secret-namespace", "",
		"Namespace containing cluster configuration secrets (defaults to POD_NAMESPACE)")
	flag.StringVar(&probeInterval, "probe-interval", "20s", "VMProbe scrape interval")
	flag.StringVar(&probeScrapeTimeout, "probe-scrape-timeout", "18s", "VMProbe scrape timeout")
	flag.StringVar(&probeModule, "probe-module", "http_2xx", "Blackbox exporter module name")
	flag.StringVar(&blackboxProberURL, "blackbox-prober-url", "blackbox.monitoring.svc:9115", "Blackbox exporter URL")

	zapOpts := zap.Options{
		Development: true,
	}
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()

	if clusterSecretNamespace == "" {
		clusterSecretNamespace = os.Getenv("POD_NAMESPACE")
		if clusterSecretNamespace == "" {
			clusterSecretNamespace = "blackbox-probe-controller-system"
		}
	}

	probeOpts := &options.Options{
		VMProbeNamespace:       vmprobeNamespace,
		ClusterSecretNamespace: clusterSecretNamespace,
		ProbeInterval:          probeInterval,
		ProbeScrapeTimeout:     probeScrapeTimeout,
		ProbeModule:            probeModule,
		BlackboxProberURL:      blackboxProberURL,
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("Disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	watchedNamespaces := map[string]cache.Config{
		clusterSecretNamespace: {},
		vmprobeNamespace:       {},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e44220c0.tapclap.com",
		Cache: cache.Options{
			DefaultNamespaces: watchedNamespaces,
			// Secrets only in the operator namespace (not monitoring). VMProbe is
			// omitted here so startup does not require the VictoriaMetrics CRD.
			ByObject: map[client.Object]cache.ByObject{
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						clusterSecretNamespace: {},
					},
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "Failed to start manager")
		os.Exit(1)
	}

	store := state.NewDesiredStateStore()
	vmprobeSync := controller.NewVMProbeSyncReconciler(mgr.GetClient(), mgr.GetScheme(), store, probeOpts)

	registry := cluster.NewRegistry(mgr.GetScheme(), store, func(remoteMgr ctrl.Manager, name string) error {
		return (&controller.RemoteIngressReconciler{
			Client:      remoteMgr.GetClient(),
			ClusterName: name,
			Store:       store,
			Trigger:     vmprobeSync,
		}).SetupWithManager(remoteMgr)
	}, vmprobeSync)

	if err := vmprobeSync.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "vmprobe-sync")
		os.Exit(1)
	}

	if err := (&controller.ClusterConfigReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		Registry:               registry,
		VMProbeSync:            vmprobeSync,
		ClusterSecretNamespace: clusterSecretNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Failed to create controller", "controller", "cluster-config")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Failed to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager",
		"vmprobeNamespace", vmprobeNamespace,
		"clusterSecretNamespace", clusterSecretNamespace,
	)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Failed to run manager")
		os.Exit(1)
	}
}

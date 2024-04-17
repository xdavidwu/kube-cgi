package main

import (
	"context"
	"flag"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kubecgiv1alpha1 "git.cs.nctu.edu.tw/aic/infra/kube-cgi/api/v1alpha1"
	"git.cs.nctu.edu.tw/aic/infra/kube-cgi/internal/controller"
	"git.cs.nctu.edu.tw/aic/infra/kube-cgi/internal/log"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))

	utilruntime.Must(kubecgiv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var kcgidImage string
	var pullSecretRef string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&kcgidImage, "kcgid-image", "", "kcgid image to use.")
	flag.StringVar(&pullSecretRef, "pull-secret", "", "namespace/name of imagePullSecret for kcgid image")
	opts := log.BuildZapOptions(flag.CommandLine)
	flag.Parse()

	if kcgidImage == "" {
		panic("kcgid image not set")
	}

	logger := zap.New(zap.UseFlagOptions(&opts))
	klog.SetLogger(logger)
	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "5e26cc2e.aic.cs.nycu.edu.tw",
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

	var pullSecret *corev1.Secret = nil
	if pullSecretRef != "" {
		parts := strings.Split(pullSecretRef, "/")
		if len(parts) != 2 {
			panic("--pull-secret not in namespace/name")
		}

		// cache of cached client (mgr.GetClient()) not started before mgr.Start()
		uncachedClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
		if err != nil {
			setupLog.Error(err, "unable to create uncached client")
			os.Exit(1)
		}

		var secret corev1.Secret
		err = uncachedClient.Get(context.Background(), client.ObjectKey{Namespace: parts[0], Name: parts[1]}, &secret)
		if err != nil {
			setupLog.Error(err, "unable to get pull secret")
			os.Exit(1)
		}
		pullSecret = &secret
	}

	if err = (&controller.APISetReconciler{
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		KcgidImage: kcgidImage,
		PullSecret: pullSecret,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "APISet")
		os.Exit(1)
	}
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = (&kubecgiv1alpha1.APISet{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "APISet")
			os.Exit(1)
		}
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

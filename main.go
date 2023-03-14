package main

import (
	"flag"
	"os"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/platform9/kube-xds/pkg/xds"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	port int

	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	// The port that this xDS server listens on
	flag.IntVar(&port, "port", 18000, "xDS management server port")
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	setupLog.Info("Setting up kube-xds...")

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 clientgoscheme.Scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		// Note: this service does not require leader election
	})

	envoyConfigClient := xds.NewConfigMapClient(mgr.GetClient())
	snapshotCache := cache.NewSnapshotCache(false, cache.IDHash{}, nil)

	if err = (&xds.ConfigMapReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConfigClient:  envoyConfigClient,
		SnapshotCache: snapshotCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "xds.ConfigMap")
		os.Exit(1)
	}

	go func() {
		setupLog.Info("XDS Server started")
		srv := server.NewServer(ctx, snapshotCache, nil)
		xds.RunServer(srv, port)
		setupLog.Info("XDS Server closed")
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

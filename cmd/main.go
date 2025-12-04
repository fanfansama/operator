package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	fgtechv1 "github.com/fgtech/ia/cursor/api/v1"
	"github.com/fgtech/ia/cursor/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme = runtime.NewScheme()
)

type envConfig struct {
	IngressHost           string
	IngressTLSSecret      string
	IngressClassName      string
	DefaultTTLSeconds     int64
	DefaultServiceAccount string
	PodPort               int32
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(fgtechv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var healthProbeAddr string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&healthProbeAddr, "health-probe-bind-address", ":8081", "The address the health probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	envCfg, err := loadEnvConfig()
	if err != nil {
		ctrl.Log.Error(err, "invalid environment configuration")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: healthProbeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "fgtech-operator",
	})
	if err != nil {
		ctrl.Log.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.FgtechReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("Fgtech"),
		IngressHost:       envCfg.IngressHost,
		IngressTLSSecret:  envCfg.IngressTLSSecret,
		IngressClassName:  envCfg.IngressClassName,
		DefaultTTLSeconds: envCfg.DefaultTTLSeconds,
		DefaultSA:         envCfg.DefaultServiceAccount,
		DefaultPodPort:    envCfg.PodPort,
	}).SetupWithManager(mgr); err != nil {
		ctrl.Log.Error(err, "unable to create controller", "controller", "Fgtech")
		os.Exit(1)
	}

	if err := mgr.Add(controllers.NewTTLWatcher(
		mgr.GetClient(),
		ctrl.Log.WithName("ttlwatcher"),
		envCfg.DefaultTTLSeconds,
		envCfg.IngressHost,
		envCfg.IngressTLSSecret,
		envCfg.IngressClassName,
	)); err != nil {
		ctrl.Log.Error(err, "unable to start ttl watcher")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		ctrl.Log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func loadEnvConfig() (envConfig, error) {
	cfg := envConfig{
		IngressHost:           os.Getenv("FGTECH_INGRESS_FQDN"),
		IngressTLSSecret:      os.Getenv("FGTECH_INGRESS_TLS_SECRET"),
		IngressClassName:      os.Getenv("FGTECH_INGRESS_CLASSNAME"),
		DefaultServiceAccount: os.Getenv("FGTECH_POD_SERVICEACCOUNT"),
		DefaultTTLSeconds:     int64(3600),
		PodPort:               8080,
	}

	if cfg.IngressHost == "" {
		return cfg, fmt.Errorf("FGTECH_INGRESS_FQDN missing")
	}
	if cfg.IngressClassName == "" {
		return cfg, fmt.Errorf("FGTECH_INGRESS_CLASSNAME missing")
	}
	if cfg.IngressTLSSecret == "" {
		cfg.IngressTLSSecret = "fgtech-tls"
	}
	if cfg.DefaultServiceAccount == "" {
		cfg.DefaultServiceAccount = "default"
	}

	if v := os.Getenv("FGTECH_DEFAULT_TTL_SECONDS"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil || parsed <= 0 {
			return cfg, fmt.Errorf("invalid FGTECH_DEFAULT_TTL_SECONDS: %s", v)
		}
		cfg.DefaultTTLSeconds = parsed
	}

	if v := os.Getenv("FGTECH_POD_PORT"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 32)
		if err != nil || parsed <= 0 || parsed > 65535 {
			return cfg, fmt.Errorf("invalid FGTECH_POD_PORT: %s", v)
		}
		cfg.PodPort = int32(parsed)
	}

	return cfg, nil
}

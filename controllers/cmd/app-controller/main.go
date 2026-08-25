// AppEnv controller binary for the Tenara platform.
package main

import (
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"tenara/controllers/internal/appenv"
	"tenara/controllers/internal/build"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	logger := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(logger)
	scheme := appenv.Scheme
	if err := corev1.AddToScheme(scheme); err != nil {
		logger.Error(err, "unable to add core types")
		os.Exit(1)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		logger.Error(err, "unable to add core types")
		os.Exit(1)
	}
	if err := build.AddToScheme(scheme); err != nil {
		logger.Error(err, "unable to add build types")
		os.Exit(1)
	}
	mgr, mgrErr := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if mgrErr != nil {
		logger.Error(mgrErr, "unable to start manager")
		os.Exit(1)
	}
	if err := appenv.NewReconciler(mgr).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create appenv controller")
		os.Exit(1)
	}
	if err := build.NewReconciler(mgr).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create build controller")
		os.Exit(1)
	}
	logger.Info("starting appenv+build controllers")
	if runErr := mgr.Start(ctrl.SetupSignalHandler()); runErr != nil {
		logger.Error(runErr, "controller exited")
		os.Exit(1)
	}
}

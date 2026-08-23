// AppEnv controller binary for the Tenara platform.
package main

import (
	"os"

	"tenara/controllers/internal/appenv"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	logger := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(logger)
	mgr, mgrErr := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: appenv.Scheme,
	})
	if mgrErr != nil {
		logger.Error(mgrErr, "unable to start manager")
		os.Exit(1)
	}
	if err := appenv.NewReconciler(mgr).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller")
		os.Exit(1)
	}
	logger.Info("starting appenv controller")
	if runErr := mgr.Start(ctrl.SetupSignalHandler()); runErr != nil {
		logger.Error(runErr, "controller exited")
		os.Exit(1)
	}
}

// Database binding controller binary for the Tenara platform.
package main

import (
	"os"

	"tenara/controllers/internal/database"
	"tenara/providers/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	logger := zap.New(zap.UseDevMode(true))
	ctrl.SetLogger(logger)

	dbP, dbErr := types.Databases.New("local")
	cacheP, cacheErr := types.Caches.New("local")
	stoP, stoErr := types.Storages.New("local")
	secP, secErr := types.Secrets.New("local")
	for name, wErr := range map[string]error{
		"database": dbErr, "cache": cacheErr, "storage": stoErr, "secret": secErr,
	} {
		if wErr != nil {
			logger.Error(wErr, "provider wiring failed", "kind", name)
			os.Exit(1)
		}
	}

	mgr, mgrErr := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: database.Scheme,
	})
	if mgrErr != nil {
		logger.Error(mgrErr, "unable to start manager")
		os.Exit(1)
	}

	deps := database.Deps{
		Databases: dbP,
		Caches:    cacheP,
		Storages:  stoP,
		Secrets:   secP,
	}
	if err := database.NewReconciler(mgr, deps).SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller")
		os.Exit(1)
	}
	logger.Info("starting database binding controller")
	if runErr := mgr.Start(ctrl.SetupSignalHandler()); runErr != nil {
		logger.Error(runErr, "controller exited")
		os.Exit(1)
	}
}

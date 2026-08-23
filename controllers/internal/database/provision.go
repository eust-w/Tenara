package database

import (
	"context"
	"errors"
	"fmt"

	"tenara/controllers/internal/appenv"
	"tenara/providers/types"
)

const (
	phasePending = "PENDING"
	phaseReady   = "READY"

	keyMongoURI = "MONGODB_URI"
	keyRedisURI = "REDIS_URI"
	keyS3Target = "S3_TARGET"
)

// Deps narrows the provider surface consumed by provisioning; nothing here
// knows about concrete local/baidu implementations (RB§40).
type Deps struct {
	Databases types.DatabaseProvider
	Caches    types.CacheProvider
	Storages  types.StorageProvider
	Secrets   types.SecretProvider
}

func secretKey(kind BindingKind) (string, error) {
	switch kind {
	case KindMongo:
		return keyMongoURI, nil
	case KindRedis:
		return keyRedisURI, nil
	case KindStorage:
		return keyS3Target, nil
	default:
		return "", fmt.Errorf("unknown binding kind %q", kind)
	}
}

// Provisioned carries the outcome handed back to the reconciler for status.
type Provisioned struct {
	Namespace  string
	SecretName string
	SecretKey  string
}

// ProvisionBinding executes the full RB§19 chain for one binding:
// provider credential -> seal through SecretProvider -> namespace injection.
// Every failure returns an error so controller-runtime retries with backoff;
// no branch ever skips the sealing step on the way to a Secret.
func ProvisionBinding(ctx context.Context, deps Deps, spec DatabaseBindingSpec) (*Provisioned, error) {
	key, err := secretKey(spec.Kind)
	if err != nil {
		return nil, err
	}

	var cred *types.Credential
	switch spec.Kind {
	case KindMongo:
		if deps.Databases == nil {
			return nil, errors.New("database provider not wired")
		}
		cred, err = deps.Databases.CreateAppDatabase(ctx, spec.AppID)
	case KindRedis:
		if deps.Caches == nil {
			return nil, errors.New("cache provider not wired")
		}
		cred, err = deps.Caches.CreateAppCache(ctx, spec.AppID)
	case KindStorage:
		if deps.Storages == nil {
			return nil, errors.New("storage provider not wired")
		}
		cred, err = deps.Storages.CreateAppStorage(ctx, spec.AppID)
	}
	if err != nil {
		return nil, fmt.Errorf("provision %s: %w", spec.Kind, err)
	}
	if cred == nil || cred.URI == "" {
		return nil, fmt.Errorf("provider returned empty credential for %s", spec.Kind)
	}

	sealed, encErr := deps.Secrets.Encrypt(ctx, []byte(cred.URI))
	if encErr != nil {
		return nil, fmt.Errorf("seal credential: %w", encErr)
	}

	ns := appenv.NamespaceName(spec.AppID, spec.Env)
	injectErr := deps.Secrets.InjectToNamespace(ctx, spec.AppID, ns,
		map[string][]byte{key: sealed})
	if injectErr != nil {
		return nil, fmt.Errorf("inject %s: %w", key, injectErr)
	}

	return &Provisioned{
		Namespace:  ns,
		SecretName: secretName(spec.AppID),
		SecretKey:  key,
	}, nil
}

// secretName mirrors the app-<id>-secrets contract shared with the secret
// provider's namespace-injection target.
func secretName(appID string) string {
	return "app-" + appID + "-secrets"
}

// DesiredPhase reports the terminal phase after a successful provisioning.
func DesiredPhase() string { return phaseReady }

// PendingPhase reports the initial reconciliation phase.
func PendingPhase() string { return phasePending }

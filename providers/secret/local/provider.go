// Package local implements SecretProvider with an AES-256-GCM KMS stub and
// direct tenant-namespace injection through the Kubernetes API (RB§22 R11).
// The controller owns the call; the HTTP API layer never touches plaintext.
package local

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"tenara/providers/types"
)

const (
	masterKeyEnv    = "TENARA_MASTER_KEY"
	labelManagedBy  = "tenara.io/managed-by"
	labelManagedVal = "tenara"
	labelAppID      = "tenara.io/app-id"
)

// SecretName derives the per-app target secret name app-<id>-secrets.
func SecretName(appID string) string { return "app-" + appID + "-secrets" }

// Provider seals platform secrets and injects them into tenant namespaces.
type Provider struct {
	sealer    *Sealer
	clientset kubernetes.Interface
}

// New wires a provider around the given Kubernetes client.
func New(cs kubernetes.Interface) (*Provider, error) {
	raw, err := loadMasterKey()
	if err != nil {
		return nil, err
	}
	sealer, sErr := NewSealer(raw)
	if sErr != nil {
		return nil, sErr
	}
	return &Provider{sealer: sealer, clientset: cs}, nil
}

func loadMasterKey() ([]byte, error) {
	value := os.Getenv(masterKeyEnv)
	if value == "" {
		return nil, fmt.Errorf("env %s is required (%d hex chars = AES-256)",
			masterKeyEnv, 2*aes256KeySize)
	}
	raw, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be hex: %w", masterKeyEnv, err)
	}
	return raw, nil
}

func (p *Provider) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	return p.sealer.Encrypt(plaintext)
}

func (p *Provider) Decrypt(_ context.Context, sealed []byte) ([]byte, error) {
	return p.sealer.Decrypt(sealed)
}

// InjectToNamespace decrypts every sealed entry and upserts them as the
// single Opaque secret app-<id>-secrets inside the target namespace only.
func (p *Provider) InjectToNamespace(ctx context.Context, appID, namespace string, secrets map[string][]byte) error {
	data := make(map[string][]byte, len(secrets))
	for key, sealed := range secrets {
		plain, dErr := p.sealer.Decrypt(sealed)
		if dErr != nil {
			return fmt.Errorf("decrypt %q: %w", key, dErr)
		}
		data[key] = plain
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(appID),
			Namespace: namespace,
			Labels: map[string]string{
				labelManagedBy: labelManagedVal,
				labelAppID:     appID,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	_, createErr := p.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if createErr == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(createErr) {
		return fmt.Errorf("create secret %s/%s: %w", namespace, secret.Name, createErr)
	}
	if _, updErr := p.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{}); updErr != nil {
		return fmt.Errorf("update secret %s/%s: %w", namespace, secret.Name, updErr)
	}
	return nil
}

// ReEncrypt rotates every revision: decrypt with the current key, seal again
// under newSealer.
func (p *Provider) ReEncrypt(entries map[string][]byte, newSealer *Sealer) (map[string][]byte, error) {
	out := make(map[string][]byte, len(entries))
	for key, sealed := range entries {
		plain, dErr := p.sealer.Decrypt(sealed)
		if dErr != nil {
			return nil, fmt.Errorf("decrypt %q: %w", key, dErr)
		}
		reSealed, eErr := newSealer.Encrypt(plain)
		if eErr != nil {
			return nil, fmt.Errorf("re-seal %q: %w", key, eErr)
		}
		out[key] = reSealed
	}
	return out, nil
}

type failingProvider struct{ reason error }

func (f failingProvider) Encrypt(context.Context, []byte) ([]byte, error) {
	return nil, f.reason
}

func (f failingProvider) Decrypt(context.Context, []byte) ([]byte, error) {
	return nil, f.reason
}

func (f failingProvider) InjectToNamespace(context.Context, string, string, map[string][]byte) error {
	return f.reason
}

func init() {
	types.Secrets.Register("local", func() types.SecretProvider {
		p, err := NewFromEnv()
		if err != nil {
			return failingProvider{reason: err}
		}
		return p
	})
}

// NewFromEnv resolves TENARA_MASTER_KEY plus an in-cluster or kubeconfig
// client; used by the registry factory outside unit tests.
func NewFromEnv() (*Provider, error) {
	cfg, inErr := rest.InClusterConfig()
	if inErr != nil {
		kubeconfig := filepath.Join(homeDir(), ".kube", "config")
		var cErr error
		cfg, cErr = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if cErr != nil {
			return nil, fmt.Errorf("no cluster access: %w", errors.Join(inErr, cErr))
		}
	}
	cs, cErr := kubernetes.NewForConfig(cfg)
	if cErr != nil {
		return nil, fmt.Errorf("clientset: %w", cErr)
	}
	return New(cs)
}

func homeDir() string {
	if home, hErr := os.UserHomeDir(); hErr == nil {
		return home
	}
	return ""
}

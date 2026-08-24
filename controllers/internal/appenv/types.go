// Package appenv implements the tenant runtime factory controller (RB-14 RB-17).
package appenv

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var Scheme = runtime.NewScheme()

// GroupVersion identifies the tenara.io/v1 API group served by this package.
var GroupVersion = schema.GroupVersion{Group: "tenara.io", Version: "v1"}

func init() {
	Scheme.AddKnownTypes(GroupVersion, &AppEnv{}, &AppEnvList{})
	metav1.AddToGroupVersion(Scheme, GroupVersion)
}

// AddToScheme registers this package's types into s.
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &AppEnv{}, &AppEnvList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

const (
	APIVersion = "tenara.io/v1"
	Kind       = "AppEnv"
)

type QuotaTier string

const (
	QuotaFree QuotaTier = "free"
	QuotaPro  QuotaTier = "pro"
)

type IsolationLevel string

const (
	IsolationShared    IsolationLevel = "shared"
	IsolationIsolated  IsolationLevel = "isolated"
	IsolationDedicated IsolationLevel = "dedicated"
)

type AppEnvSpec struct {
	AppID      string         `json:"appId"`
	Env        string         `json:"env"`
	AppSpecRef string         `json:"appspecRef,omitempty"`
	DomainRefs []string       `json:"domainRefs,omitempty"`
	QuotaTier  QuotaTier      `json:"quotaTier"`
	Isolation  IsolationLevel `json:"isolation"`
}

type AppEnvStatus struct {
	Namespace string `json:"namespace,omitempty"`
	Phase     string `json:"phase,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type AppEnv struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppEnvSpec   `json:"spec,omitempty"`
	Status AppEnvStatus `json:"status,omitempty"`
}

type AppEnvList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppEnv `json:"items"`
}

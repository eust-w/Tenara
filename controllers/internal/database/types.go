// Package database orchestrates per-app credential provisioning end-to-end:
// DatabaseProvider -> sealed via SecretProvider -> tenant-namespace Secret
// (RB§19 chain; admin credentials stay inside provider processes).
package database

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var Scheme = runtime.NewScheme()

const (
	APIVersion = "tenara.io/v1"
	Kind       = "DatabaseBinding"
)

// BindingKind selects which data-plane capability the app needs.
type BindingKind string

const (
	KindMongo   BindingKind = "mongo"
	KindRedis   BindingKind = "redis"
	KindStorage BindingKind = "storage"
)

type DatabaseBindingSpec struct {
	AppID string      `json:"appId"`
	Env   string      `json:"env"`
	Kind  BindingKind `json:"kind"`
}

type DatabaseBindingStatus struct {
	Phase   string `json:"phase,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type DatabaseBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseBindingSpec   `json:"spec,omitempty"`
	Status DatabaseBindingStatus `json:"status,omitempty"`
}

func (*DatabaseBinding) DeepCopyObject() runtime.Object { return nil }

type DatabaseBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DatabaseBinding `json:"items"`
}

func (*DatabaseBindingList) DeepCopyObject() runtime.Object { return nil }

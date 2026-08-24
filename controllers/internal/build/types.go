// Package build implements the Tenara build-plane controller (RB-12 RB-34).
package build

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var Scheme = runtime.NewScheme()

// GroupVersion identifies the tenara.io/v1 API group served by this package.
var GroupVersion = schema.GroupVersion{Group: "tenara.io", Version: "v1"}

// AddToScheme registers this package's types into s.
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &Build{}, &BuildList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

func init() {
	Scheme.AddKnownTypes(GroupVersion, &Build{}, &BuildList{})
	metav1.AddToGroupVersion(Scheme, GroupVersion)
}

const (
	APIVersion = "tenara.io/v1"
	KindBuild  = "Build"
)

type BuildPhase string

const (
	PhaseCreated  BuildPhase = "CREATED"
	PhaseCloning  BuildPhase = "CLONING"
	PhaseBuilding BuildPhase = "BUILDING"
	PhaseScanning BuildPhase = "SCANNING"
	PhaseSigning  BuildPhase = "SIGNING"
	PhasePushed   BuildPhase = "PUSHED"
	PhaseFailed   BuildPhase = "FAILED"
)

type GitSource struct {
	URL      string `json:"url"`
	SHA      string `json:"sha,omitempty"`
	TokenRef string `json:"tokenRef,omitempty"`
}

type BuildSpec struct {
	AppID      string    `json:"app"`
	Env        string    `json:"env"`
	Git        GitSource `json:"git"`
	AppSpecRef string    `json:"appspecRef,omitempty"`
	Dockerfile string    `json:"dockerfile,omitempty"`
}

type BuildStatus struct {
	Phase         BuildPhase `json:"phase,omitempty"`
	ImageDigest   string     `json:"digest,omitempty"`
	SBOMRef       string     `json:"sbom,omitempty"`
	ScanReportRef string     `json:"scan,omitempty"`
	SignatureRef  string     `json:"sign,omitempty"`
	Reason        string     `json:"reason,omitempty"`
	Message       string     `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec,omitempty"`
	Status BuildStatus `json:"status,omitempty"`
}

type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Build `json:"items"`
}

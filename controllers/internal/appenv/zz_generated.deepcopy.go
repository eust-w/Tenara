// Code generated-style deepcopy for Tenara CRDs (controller-gen pending).
// Hand-written during post-plan hardening; round-trip tests guard drift.
package appenv

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out.
func (in *AppEnvSpec) DeepCopyInto(out *AppEnvSpec) {
	*out = *in
	if in.DomainRefs != nil {
		out.DomainRefs = append(out.DomainRefs[:0:0], in.DomainRefs...)
	}
	if in.Services != nil {
		out.Services = make([]ServiceSpec, len(in.Services))
		copy(out.Services, in.Services)
	}
}

func (in *AppEnvSpec) DeepCopy() *AppEnvSpec {
	if in == nil {
		return nil
	}
	out := new(AppEnvSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *AppEnvStatus) DeepCopyInto(out *AppEnvStatus) {
	*out = *in
}

func (in *AppEnvStatus) DeepCopy() *AppEnvStatus {
	if in == nil {
		return nil
	}
	out := new(AppEnvStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *AppEnv) DeepCopyInto(out *AppEnv) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy returns a deep copy of the receiver.
func (in *AppEnv) DeepCopy() *AppEnv {
	if in == nil {
		return nil
	}
	out := new(AppEnv)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as runtime.Object.
func (in *AppEnv) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	return in.DeepCopy()
}

// DeepCopyInto copies the receiver into out.
func (in *AppEnvList) DeepCopyInto(out *AppEnvList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]AppEnv, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the receiver.
func (in *AppEnvList) DeepCopy() *AppEnvList {
	if in == nil {
		return nil
	}
	out := new(AppEnvList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as runtime.Object.
func (in *AppEnvList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	return in.DeepCopy()
}

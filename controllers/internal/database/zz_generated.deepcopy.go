// Code generated-style deepcopy for Tenara CRDs (controller-gen pending).
// Hand-written during post-plan hardening; round-trip tests guard drift.
package database

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out.
func (in *DatabaseBindingSpec) DeepCopyInto(out *DatabaseBindingSpec) {
	*out = *in
}

func (in *DatabaseBindingSpec) DeepCopy() *DatabaseBindingSpec {
	if in == nil {
		return nil
	}
	out := new(DatabaseBindingSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *DatabaseBindingStatus) DeepCopyInto(out *DatabaseBindingStatus) {
	*out = *in
}

func (in *DatabaseBindingStatus) DeepCopy() *DatabaseBindingStatus {
	if in == nil {
		return nil
	}
	out := new(DatabaseBindingStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *DatabaseBinding) DeepCopyInto(out *DatabaseBinding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy returns a deep copy of the receiver.
func (in *DatabaseBinding) DeepCopy() *DatabaseBinding {
	if in == nil {
		return nil
	}
	out := new(DatabaseBinding)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as runtime.Object.
func (in *DatabaseBinding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *DatabaseBindingList) DeepCopyInto(out *DatabaseBindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]DatabaseBinding, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep copy of the receiver.
func (in *DatabaseBindingList) DeepCopy() *DatabaseBindingList {
	if in == nil {
		return nil
	}
	out := new(DatabaseBindingList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a deep copy as runtime.Object.
func (in *DatabaseBindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

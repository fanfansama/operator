package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group   = "fgtech.fgtech.io"
	Version = "v1"
)

var (
	GroupVersion  = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

// FgtechSpec defines the desired state of Fgtech
type FgtechSpec struct {
	Version        string `json:"version"`
	Image          string `json:"image"`
	ExtraPath      string `json:"extrapath,omitempty"`
	TTLSeconds     *int64 `json:"ttlSeconds,omitempty"`
	ServiceAccount string `json:"serviceaccount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=fgteches,scope=Namespaced
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Image",type=string,JSONPath=`.spec.image`
// +kubebuilder:printcolumn:name="ExtraPath",type=string,JSONPath=`.spec.extrapath`
// +kubebuilder:printcolumn:name="TTL",type=integer,JSONPath=`.spec.ttlSeconds`
// +kubebuilder:printcolumn:name="ServiceAccount",type=string,JSONPath=`.spec.serviceaccount`
type Fgtech struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FgtechSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
type FgtechList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Fgtech `json:"items"`
}

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &Fgtech{}, &FgtechList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

func (in *Fgtech) DeepCopyInto(out *Fgtech) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

func (in *Fgtech) DeepCopy() *Fgtech {
	if in == nil {
		return nil
	}
	out := new(Fgtech)
	in.DeepCopyInto(out)
	return out
}

func (in *Fgtech) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

func (in *FgtechList) DeepCopyInto(out *FgtechList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Fgtech, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (in *FgtechList) DeepCopy() *FgtechList {
	if in == nil {
		return nil
	}
	out := new(FgtechList)
	in.DeepCopyInto(out)
	return out
}

func (in *FgtechList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// APISetSpec defines the desired state of APISet
type APISetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of APISet. Edit apiset_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// APISetStatus defines the observed state of APISet
type APISetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// APISet is the Schema for the apisets API
type APISet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APISetSpec   `json:"spec,omitempty"`
	Status APISetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// APISetList contains a list of APISet
type APISetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APISet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APISet{}, &APISetList{})
}

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Request struct {
	// JSON Schema to validate requests with
	// TODO validate this with webhook
	// TODO consider YAML serialization support a la OpenAPI
	Schema string `json:"schema,omitempty"`
}

type Response struct {
	// Content-Type to fill for responses
	//+kubebuilder:default=application/json
	ContentType string `json:"contentType,omitempty"`
}

// A Pod is retained when it statisfies all specified rules
// TODO impl
type HistoryLimitSpec struct {
	// Retain at most this number of pods
	MaxCount *int32 `json:"maxCount,omitempty"`

	// Pod with terminated time under this range should be retained
	MaxAge *int32 `json:"maxAge,omitempty"`

	// Pod from older version of this APISet should be retained
	//+kubebuilder:default=false
	KeepPreviousVersions *bool `json:"includePreviousVersions,omitempty"`
}

// Policies to retain historic pods
type HistoryLimit struct {
	Success HistoryLimitSpec `json:"success,omitempty"`
	Failure HistoryLimitSpec `json:"failure,omitempty"`
}

type API struct {
	// Path of this API endpoint
	//+kubebuilder:validation:Format=uri
	Path string `json:"path"`

	// Spec of the pod,
	// only one container expected, restartPolicy should be Never
	corev1.PodSpec `json:"podSpec"`

	*Request  `json:"request,omitempty"`
	*Response `json:"response,omitempty"`
}

// Deployment settings of the distributed API runtime
type DAPI struct {
	//+kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// TODO consider what may be customized

	// monitoring.coreos.com/v1 ServiceMonitor support
	// TODO impl
	ServiceMonitor *bool `json:"serviceMonitor,omitempty"`
}

// APISetSpec defines the desired state of APISet
type APISetSpec struct {
	// The domain name this APISet should serve on
	//+kubebuilder:validation:Format=hostname
	Host string `json:"host"`

	// The APIs to host under the specified domain name
	APIs []API `json:"apis"`

	*DAPI `json:"dapi,omitempty"`

	// Hoist the images onto nodes with DaemonSets
	// The image is expected to contain a `true` command
	// TODO impl
	//+kubebuilder:default=false
	HoistImages *bool `json:"hoistImages,omitempty"`

	*HistoryLimit `json:"historyLimit,omitempty"`
}

// APISetStatus defines the observed state of APISet
type APISetStatus struct {
	ServiceAccount  *corev1.ObjectReference `json:"serviceAccount,omitempty"`
	RoleBinding     *corev1.ObjectReference `json:"roleBinding,omitempty"`
	Deployment      *corev1.ObjectReference `json:"deployment,omitempty"`
	Service         *corev1.ObjectReference `json:"service,omitempty"`
	Ingress         *corev1.ObjectReference `json:"ingress,omitempty"`
	ImagePullSecret *corev1.ObjectReference `json:"imagePullSecret,omitempty"`
	Deployed        *bool                   `json:"deployed,omitempty"`
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
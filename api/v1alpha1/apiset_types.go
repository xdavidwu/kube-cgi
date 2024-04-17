package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Type=object
// +kubebuilder:pruning:PreserveUnknownFields
// +kubebuilder:object:generate=false
type Schema struct {
	RawJSON string
}

func (s *Schema) MarshalJSON() ([]byte, error) {
	return []byte(s.RawJSON), nil
}

func (s *Schema) UnmarshalJSON(b []byte) error {
	s.RawJSON = string(b)
	return nil
}

type Request struct {
	// JSON Schema to validate requests with, as an inline object.
	// Empty object may be used to enforce being JSON only.
	Schema *Schema `json:"schema,omitempty"`
}

type Response struct {
	// TODO define CGI script failure behavior
	// TODO also consider making this setable per-apiset
}

// A Pod is retained when it statisfies all specified rules
type HistoryLimitSpec struct {
	// Retain at most this number of pods from current version.
	// Defaults to 0 on succeeded, 5 on failed.
	MaxCount *int32 `json:"maxCount,omitempty"`

	// Retain pods from current version for at most this amount of seconds
	// TODO impl
	MaxAge *int32 `json:"maxAge,omitempty"`

	// Pod from older version of this APISet should be retained
	//+kubebuilder:default=false
	KeepPreviousVersions *bool `json:"keepPreviousVersions,omitempty"`
}

// Policies to retain historic pods
type HistoryLimit struct {
	Succeeded HistoryLimitSpec `json:"succeeded,omitempty"`
	Failed    HistoryLimitSpec `json:"failed,omitempty"`
}

type API struct {
	// Path of this API endpoint.
	// /readyz is reserved for internal readiness checks.
	//+kubebuilder:validation:Format=uri
	Path string `json:"path"`

	// Spec of the pod.
	// Only one container expected, restartPolicy must be Never.
	// If stdin of the container is true, stdinOnce must also be true,
	// where request body will also be sent to stdin.
	//+kubebuilder:validation:XValidation:message="Only one container in each podSpec is allowed",rule="self.containers.size() == 1"
	//+kubebuilder:validation:XValidation:message="Container with stdin must also set stdinOnce",rule="!has(self.containers[0].stdin) || self.containers[0].stdin != true || self.containers[0].stdinOnce == true"
	//+kubebuilder:validation:XValidation:message="restartPolicy must be Never",rule="self.restartPolicy == 'Never' && (!has(self.containers[0].restartPolicy) || self.containers[0].restartPolicy == 'Never')"
	//+kubebuilder:validation:XValidation:message="initContainers is not supported",rule="!has(self.initContainers) || self.initContainers.size() == 0"
	//+kubebuilder:validation:XValidation:message="ephemeralContainers is not supported",rule="!has(self.ephemeralContainers)"
	corev1.PodSpec `json:"podSpec"`

	*Request  `json:"request,omitempty"`
	*Response `json:"response,omitempty"`
}

// Deployment settings of the distributed API runtime
type DAPI struct {
	//+kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	Args []string `json:"args,omitempty"`

	// Create monitoring.coreos.com/v1 ServiceMonitor for distributed API
	// runtime metrics
	//+kubebuilder:default=false
	ServiceMonitor bool `json:"serviceMonitor,omitempty"`
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
	ServiceMonitor  *corev1.ObjectReference `json:"serviceMonitor,omitempty"`
	Deployed        *bool                   `json:"deployed,omitempty"`

	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Host",type="string",JSONPath=".spec.host",description="Host name this APISet is served under"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp is a timestamp representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations. Clients may not set this value. It is represented in RFC3339 form and is in UTC. Populated by the system. Read-only. Null for lists. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata"

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

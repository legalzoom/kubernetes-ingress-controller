package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KongService is the Schema for the kongservices API.
// +genclient
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=kong-ingress-controller
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
type KongService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KongServiceSpec   `json:"spec,omitempty"`
	Status KongServiceStatus `json:"status,omitempty"`
}

// KongServiceList contains a list of KongService.
// +kubebuilder:object:root=true
type KongServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KongService `json:"items"`
}

type KongServiceSpec struct {
}

type KongServiceStatus struct {
}

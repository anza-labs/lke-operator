/*
Copyright 2024 lke-operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LKEClusterConfigSpec defines the desired state of an LKEClusterConfig resource.
type LKEClusterConfigSpec struct {
	// Region is the geographical region where the LKE cluster will be provisioned.
	// +kubebuilder:validation:required
	Region string `json:"region"`

	// TokenSecretRef references the Kubernetes secret that stores the Linode API token.
	// If not provided, then default token will be used.
	// +kubebuilder:validation:required
	TokenSecretRef SecretRef `json:"tokenSecretRef"`

	// HighAvailability specifies whether the LKE cluster should be configured for high
	// availability.
	// +kubebuilder:validation:optional
	// +kubebuilder:default=false
	HighAvailability *bool `json:"highAvailability,omitempty"`

	// NodePools contains the specifications for each node pool within the LKE cluster.
	// +kubebuilder:validation:required
	// +kubebuilder:validation:minProperties=1
	NodePools map[string]LKENodePool `json:"nodePools"`

	// KubernetesVersion indicates the Kubernetes version of the LKE cluster.
	// +kubebuilder:validation:optional
	// +kubebuilder:default=latest
	KubernetesVersion *string `json:"kubernetesVersion,omitempty"`
}

// SecretRef references a Kubernetes secret.
type SecretRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// LKENodePool represents a pool of nodes within the LKE cluster.
type LKENodePool struct {
	// NodeCount specifies the number of nodes in the node pool.
	// +kubebuilder:validation:required
	NodeCount int `json:"nodeCount"`

	// LinodeType specifies the Linode instance type for the nodes in the pool.
	// +kubebuilder:validation:required
	LinodeType string `json:"linodeType"`

	// Autoscaler specifies the autoscaling configuration for the node pool.
	// +kubebuilder:validation:optional
	Autoscaler *LKENodePoolAutoscaler `json:"autoscaler,omitempty"`
}

func (l LKENodePool) IsEqual(cmp LKENodePool) bool {
	if l.NodeCount != cmp.NodeCount {
		return false
	}

	if l.LinodeType != cmp.LinodeType {
		return false
	}

	if l.Autoscaler == nil && cmp.Autoscaler == nil {
		return true
	}

	if l.Autoscaler == nil || cmp.Autoscaler == nil {
		return false
	}

	return l.Autoscaler.Min == cmp.Autoscaler.Min && l.Autoscaler.Max == cmp.Autoscaler.Max
}

// LKENodePoolAutoscaler represents the autoscaler configuration for a node pool.
type LKENodePoolAutoscaler struct {
	// Min specifies the minimum number of nodes in the pool.
	// +kubebuilder:validation:required
	// +kubebuilder:validation:minimum=0
	// +kubebuilder:validation:maximum=100
	Min int `json:"min"`

	// Max specifies the maximum number of nodes in the pool.
	// +kubebuilder:validation:required
	// +kubebuilder:validation:minimum=3
	// +kubebuilder:validation:maximum=100
	Max int `json:"max"`
}

// LKEClusterConfigStatus defines the observed state of an LKEClusterConfig resource.
type LKEClusterConfigStatus struct {
	// Phase represents the current phase of the LKE cluster.
	// +kubebuilder:validation:optional
	// +kubebuilder:default=Unknown
	Phase *Phase `json:"phase,omitempty"`

	// ClusterID contains the ID of the provisioned LKE cluster.
	// +kubebuilder:validation:optional
	ClusterID *int `json:"clusterID,omitempty"`

	// NodePoolStatuses contains the Status of the provisioned node pools within the LKE cluster.
	// +kubebuilder:validation:optional
	NodePoolStatuses map[string]NodePoolStatus `json:"nodePoolStatuses,omitempty"`

	// FailureMessage contains an optional failure message for the LKE cluster.
	// +kubebuilder:validation:optional
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// NodePoolStatus
type NodePoolStatus struct {
	// ID
	// +kubebuilder:validation:optional
	ID *int `json:"id,omitempty"`

	// NodePoolDetails
	// +kubebuilder:validation:required
	NodePoolDetails LKENodePool `json:"details"`
}

func (n NodePoolStatus) IsEqual(cmp NodePoolStatus) bool {
	return n.NodePoolDetails.IsEqual(cmp.NodePoolDetails)
}

// +kubebuilder:validation:Enum=Active;Deleting;Provisioning;Unknown;Updating
type Phase string

const (
	PhaseActive       Phase = "Active"
	PhaseDeleting     Phase = "Deleting"
	PhaseProvisioning Phase = "Provisioning"
	PhaseUpdating     Phase = "Updating"
	PhaseUnknown      Phase = "Unknown"
)

// +kubebuilder:object:root=true

// LKEClusterConfig is the Schema for the lkeclusterconfigs API.
// +kubebuilder:resource:shortName=lkecc
// +kubebuilder:printcolumn:name=Region,type=string,JSONPath=`.spec.region`
// +kubebuilder:printcolumn:name=K8sVersion,type=string,JSONPath=`.spec.kubernetesVersion`
// +kubebuilder:printcolumn:name=Phase,type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name=FailureMessage,type=string,JSONPath=`.status.failureMessage`
type LKEClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LKEClusterConfigSpec   `json:"spec,omitempty"`
	Status LKEClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LKEClusterConfigList contains a list of LKEClusterConfig
type LKEClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LKEClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LKEClusterConfig{}, &LKEClusterConfigList{})
}

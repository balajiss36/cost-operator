/*
Copyright 2024.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CostOptimizerSpec defines the desired state of CostOptimizer
type CostOptimizerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of CostOptimizer. Edit costoptimizer_types.go to remove/update
	Name     string `json:"name,omitempty"`
	Object   string `json:"object,omitempty"`
	Schedule string `json:"schedule,omitempty"`
}

// CostOptimizerStatus defines the observed state of CostOptimizer
type CostOptimizerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status           string       `json:"status,omitempty"`
	Active           string       `json:"active,omitempty"`
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CostOptimizer is the Schema for the costoptimizers API
type CostOptimizer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CostOptimizerSpec   `json:"spec,omitempty"`
	Status CostOptimizerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CostOptimizerList contains a list of CostOptimizer
type CostOptimizerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CostOptimizer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CostOptimizer{}, &CostOptimizerList{})
}

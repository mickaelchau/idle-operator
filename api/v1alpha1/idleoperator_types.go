/*
Copyright 2021.

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

type IdlingSpecs struct {
	MatchingLabels []map[string]string `json:"matchLabels"`
	Time           string              `json:"time"`
	Duration       string              `json:"duration"`
	Namespace      string              `json:"namespace"`
}
type StatusDeployment struct {
	Name    string `json:"name"`
	Size    int32  `json:"size"`
	IsIdled bool   `json:"isIdled"`
}

// IddleDeployFromCrontableSpec defines the desired state of IddleDeployFromCrontable
type IdleOperatorSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of IddleDeployFromCrontable. Edit iddledeployfromcrontable_types.go to remove/update
	Idle []IdlingSpecs `json:"idle"`
}

// IddleDeployFromCrontableStatus defines the observed state of IddleDeployFromCrontable
type IdleOperatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	StatusDeployments []StatusDeployment `json:"deployments"` //dictionary
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// IddleDeployFromCrontable is the Schema for the iddledeployfromcrontables API
type IdleOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdleOperatorSpec   `json:"spec,omitempty"`
	Status IdleOperatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IddleDeployFromCrontableList contains a list of IddleDeployFromCrontable
type IdleOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IdleOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IdleOperator{}, &IdleOperatorList{})
}

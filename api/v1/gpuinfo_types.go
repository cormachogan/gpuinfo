/*


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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUInfoSpec defines the desired state of GPUInfo
type GPUInfoSpec struct {
	DesAccTime  int64 `json:"desAccTime"`
	GPURequired bool  `json:"gpuRequired"`
}

// GPUInfoStatus defines the observed state of GPUInfo
type GPUInfoStatus struct {
	SuitableNodeName         string `json:"suitableNodeName"`
	SuitableHostName         string `json:"suitableHostName"`
	NodeMemoryUsage          int64  `json:"nodeMemoryUsage"`
	NodeCPUUsage             int64  `json:"nodeCPUUsage"`
	AvailableAcceleratorTime int64  `json:"availableAcceleratorTime"`
}

// +kubebuilder:validation:Optional
// +kubebuilder:resource:shortName={"gpu"}
// +kubebuilder:printcolumn:name="Desired Access Time (Hrs)",type=integer,JSONPath=`.spec.desAccTime`
// +kubebuilder:printcolumn:name="GPU Required",type=boolean,JSONPath=`.spec.gpuRequired`
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GPUInfo is the Schema for the gpuinfoes API
type GPUInfo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUInfoSpec   `json:"spec,omitempty"`
	Status GPUInfoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPUInfoList contains a list of GPUInfo
type GPUInfoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPUInfo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPUInfo{}, &GPUInfoList{})
}

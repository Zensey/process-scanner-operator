/*
Copyright 2025.

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

// NodeProcessScanSpec defines the processes to scan for
type NodeProcessScanSpec struct {
	// Processes to detect (e.g., ["nginx", "redis"])
	ProcessNames []string `json:"processNames"`
	
	// Scan interval in seconds (default: 30)
	Interval int `json:"interval,omitempty"`
}

// ProcessDetection represents found processes
type ProcessDetection struct {
	ProcessName string `json:"processName"`
	PID         int    `json:"pid"`
	Command     string `json:"command,omitempty"`
}

// NodeProcessScanStatus defines the observed state
type NodeProcessScanStatus struct {
	// Key: "node/pod/container"
	Results map[string][]ProcessDetection `json:"results,omitempty"`
	
	LastScanTime  metav1.Time `json:"lastScanTime,omitempty"`
	ScanningNodes int         `json:"scanningNodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type NodeProcessScan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeProcessScanSpec   `json:"spec,omitempty"`
	Status NodeProcessScanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NodeProcessScanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeProcessScan `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeProcessScan{}, &NodeProcessScanList{})
}

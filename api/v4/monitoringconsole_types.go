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

package v4

import (
	splcommon "github.com/splunk/splunk-operator/pkg/splunk/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MonitoringConsoleSpec struct {
	CommonSplunkSpec `json:",inline"`

	// Splunk Enterprise App repository. Specifies remote App location and scope for Splunk App management
	AppFrameworkConfig AppFrameworkSpec `json:"appRepo,omitempty"`
}

// MonitoringConsoleStatus defines the observed state of MonitoringConsole
type MonitoringConsoleStatus struct {
	// current phase of the monitoring console
	Phase splcommon.Phase `json:"phase"`

	// selector for pods, used by HorizontalPodAutoscaler
	Selector string `json:"selector"`

	// Bundle push status tracker
	BundlePushTracker BundlePushInfo `json:"bundlePushInfo"`

	// Resource Revision tracker
	ResourceRevMap map[string]string `json:"resourceRevMap"`

	// App Framework status
	AppContext AppDeploymentContext `json:"appContext,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MonitoringConsole is the Schema for the monitoringconsole API
// +kubebuilder:resource:shortName=mc;mconsole
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Status of monitoring console"
// +kubebuilder:resource:path=monitoringconsoles,scope=Namespaced,shortName=mc
type MonitoringConsole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MonitoringConsoleSpec   `json:"spec,omitempty"`
	Status MonitoringConsoleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MonitoringConsoleList contains a list of MonitoringConsole
type MonitoringConsoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MonitoringConsole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MonitoringConsole{}, &MonitoringConsoleList{})
}

// NewEvent creates a new event associated with the object and ready
// to be published to the kubernetes API.
func (mcnsl *MonitoringConsole) NewEvent(eventType, reason, message string) corev1.Event {
	t := metav1.Now()
	return corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: reason + "-",
			Namespace:    mcnsl.ObjectMeta.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:       "MonitoringConsole",
			Namespace:  mcnsl.Namespace,
			Name:       mcnsl.Name,
			UID:        mcnsl.UID,
			APIVersion: GroupVersion.String(),
		},
		Reason:  reason,
		Message: message,
		Source: corev1.EventSource{
			Component: "splunk-monitoringconsole-controller",
		},
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		Type:                eventType,
		ReportingController: "enterprise.splunk.com/monitoringconsole-controller",
		//Related:             standln.Spec.ConsumerRef,
	}
}

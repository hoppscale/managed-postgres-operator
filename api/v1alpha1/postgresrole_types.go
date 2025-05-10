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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostgresRoleSpec defines the desired state of PostgresRole.
type PostgresRoleSpec struct {
	// PostgreSQL role name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:message="name is immutable",rule="self == oldSelf"
	Name string `json:"name,omitempty"`

	SuperUser   bool `json:"superUser,omitempty"`
	CreateDB    bool `json:"createDB,omitempty"`
	CreateRole  bool `json:"createRole,omitempty"`
	Inherit     bool `json:"inherit,omitempty"`
	Login       bool `json:"login,omitempty"`
	Replication bool `json:"replication,omitempty"`
	BypassRLS   bool `json:"bypassRLS,omitempty"`

	PasswordSecretName string `json:"passwordSecretName,omitempty"`
}

// PostgresRoleStatus defines the observed state of PostgresRole.
type PostgresRoleStatus struct {
	Succeeded bool `json:"succeeded"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresRole is the Schema for the postgresroles API.
type PostgresRole struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresRoleSpec   `json:"spec,omitempty"`
	Status PostgresRoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresRoleList contains a list of PostgresRole.
type PostgresRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresRole{}, &PostgresRoleList{})
}

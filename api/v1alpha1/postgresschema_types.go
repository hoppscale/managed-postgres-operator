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

// PostgresSchemaPrivilegesSpec defines the desired schema privileges to grant to roles
type PostgresSchemaPrivilegesSpec struct {
	Create bool `json:"create,omitempty"`
	Usage  bool `json:"usage,omitempty"`
}

// PostgresSchemaSpec defines the desired state of a PostgreSQL schema
type PostgresSchemaSpec struct {
	// Database is the PostgreSQL database's name in which the schema exists
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:message="database is immutable",rule="self == oldSelf"
	Database string `json:"database"`

	// Name is the PostgreSQL schema's name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:message="name is immutable",rule="self == oldSelf"
	Name string `json:"name"`

	// Owner is the PostgreSQL schema's owner. It must be a valid existing role.
	Owner string `json:"owner,omitempty"`

	// KeepOnDelete will determine if the deletion of the resource should drop the remote PostgreSQL schema. Default is false.
	KeepOnDelete bool `json:"keepOnDelete,omitempty"`

	// PrivilegesByRole will grant privileges to roles on this schema
	PrivilegesByRole map[string]PostgresSchemaPrivilegesSpec `json:"privilegesByRole,omitempty"`
}

// PostgresSchemaStatus defines the observed state of PostgresSchema.
type PostgresSchemaStatus struct {
	Succeeded bool `json:"succeeded"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresSchema is the Schema for the postgresschemas API.
type PostgresSchema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresSchemaSpec   `json:"spec,omitempty"`
	Status PostgresSchemaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresSchemaList contains a list of PostgresSchema.
type PostgresSchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresSchema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresSchema{}, &PostgresSchemaList{})
}

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

// PostgresDatabasePrivilegesSpec defines the desired database privileges to grant to roles
type PostgresDatabasePrivilegesSpec struct {
	Create    bool `json:"create,omitempty"`
	Connect   bool `json:"connect,omitempty"`
	Temporary bool `json:"temporary,omitempty"`
}

// PostgresDatabaseSpec defines the desired state of PostgresDatabase.
type PostgresDatabaseSpec struct {

	// Name is the PostgreSQL database's name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:message="name is immutable",rule="self == oldSelf"
	Name string `json:"name"`

	// Owner is the PostgreSQL database's owner. It must be a valid existing role.
	Owner string `json:"owner,omitempty"`

	// Extensions is the list of database extensions to install on the database.
	Extensions []string `json:"extensions,omitempty"`

	// KeepOnDelete will determine if the deletion of the resource should drop the remote PostgreSQL database. Default is false.
	KeepOnDelete bool `json:"keepOnDelete,omitempty"`

	// PreserveConnectionsOnDelete will determine if the deletion of the object should drop the existing connections to the remote PostgreSQL database. Default is false.
	PreserveConnectionsOnDelete bool `json:"preserveConnectionsOnDelete,omitempty"`

	// PrivilegesByRole will grant privileges to roles
	PrivilegesByRole map[string]PostgresDatabasePrivilegesSpec `json:"privilegesByRole,omitempty"`
}

// PostgresDatabaseStatus defines the observed state of PostgresDatabase.
type PostgresDatabaseStatus struct {
	Succeeded bool `json:"succeeded"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PostgresDatabase is the Schema for the postgresdatabases API.
type PostgresDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresDatabaseSpec   `json:"spec,omitempty"`
	Status PostgresDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresDatabaseList contains a list of PostgresDatabase.
type PostgresDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PostgresDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PostgresDatabase{}, &PostgresDatabaseList{})
}

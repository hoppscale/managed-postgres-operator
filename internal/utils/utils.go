package utils

import (
	"crypto/sha256"
	"fmt"
)

const OperatorInstanceAnnotationName string = "managed-postgres-operator.hoppscale.com/instance"

func IsManagedByOperatorInstance(annotations map[string]string, instanceName string) bool {
	if instance, ok := annotations[OperatorInstanceAnnotationName]; ok && instance == instanceName {
		return true
	}

	if instanceName == "" {
		return true
	}

	return false
}

func GetLeaderElectionID(instanceName string) string {
	leaderName := "default"

	if instanceName != "" {
		leaderName = instanceName
	}

	h := sha256.New()
	h.Write([]byte(leaderName))
	leaderNameHash := fmt.Sprintf("%x", h.Sum(nil))
	leaderElectionID := fmt.Sprintf("%.14s-%.8s.managed-postgres-operator.hoppscale.com", leaderName, leaderNameHash)

	return leaderElectionID
}

package utils

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

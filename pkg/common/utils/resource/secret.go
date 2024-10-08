package resource

import corev1 "k8s.io/api/core/v1"

func GetDorisLoginInformation(secret *corev1.Secret) (adminUserName, password string) {
	adminUserName = "root"
	if secret != nil && secret.Data != nil {
		if _, ok := secret.Data["username"]; ok {
			adminUserName = string(secret.Data["username"])
		}
		password = string(secret.Data["password"])
	}
	return adminUserName, password
}

package resource

import corev1 "k8s.io/api/core/v1"

func GetDorisLoginInformation(secret *corev1.Secret) (adminUserName, password string) {
	if secret != nil && secret.Data != nil {
		return string(secret.Data["username"]), string(secret.Data["password"])
	}
	return "root", ""
}

package conf

import (
	"github.com/selectdb/doris-operator/pkg/controller"
	"os"
)

var Default_Secret_Name = "doris-operator-secret-cert"

type EnvVariables struct {
	EnableWebhook     bool
	OperatorNamespace string
	OperatorName      string
	ServiceName       string
}

// get envs
func ParseEnvs() *EnvVariables {
	ev := &EnvVariables{}
	enableWebhook := os.Getenv("ENABLE_WEBHOOK")
	if enableWebhook == "true" {
		ev.EnableWebhook = true
	}

	ev.OperatorNamespace = os.Getenv("OPERATOR_NAMESPACE")
	if ev.OperatorNamespace == "" {
		ev.OperatorNamespace = "doris"
	}
	ev.OperatorName = os.Getenv("OPERATOR_NAME")
	if ev.OperatorName == "" {
		ev.OperatorName = "doris-operator"
	}

	ev.ServiceName = os.Getenv("SERVICE_NAME")
	if ev.ServiceName == "" {
		ev.ServiceName = "doris-operator-service"
	}
	return ev
}

// build start parameters for controller
func NewControllerOptions(envs *EnvVariables) *controller.Options {
	return &controller.Options{
		EnableWebHook:  envs.EnableWebhook,
		Name:           envs.OperatorName,
		SecretName:     Default_Secret_Name,
		Namespace:      envs.OperatorNamespace,
		WebhookService: envs.ServiceName,
	}
}

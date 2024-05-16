package controller

// operator start options
type Options struct {
	EnableWebHook bool
	// the operator name
	Name string
	//the secret name
	SecretName string
	// namespace of operator deployed.
	Namespace string
	//the service for operator
	WebhookService string
}

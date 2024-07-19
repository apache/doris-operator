{{- define "operator.serviceAccountName" -}}
{{- print "doris-operator" }}
{{- end }}

{{- define "operator.container.name" -}}
{{- print "dorisoperator" }}
{{- end }}

{{- define "operator.namespace" -}}
{{ print .Release.Namespace }}
{{- end }}

{{- define "kube-doris.name" -}}
{{- print "doris" }}
{{- end }}

{{/*
doris operator webhook open.
*/}}
{{- define "webhook.enable" -}}
{{ default "false" .Values.dorisOperator.enableWebhook }}
{{- end -}}


{{/*
doris operator webhook service name.
*/}}
{{- define "webhook.serviceName" }}
{{- print "doris-operator-service" }}
{{- end }}

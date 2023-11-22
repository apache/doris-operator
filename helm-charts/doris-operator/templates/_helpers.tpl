{{- define "operator.serviceAccountName" -}}
{{- print "doris-operator" }}
{{- end }}

{{- define "operator.container.name" -}}
{{- print "dorisoperator" }}
{{- end }}

{{- define "operator.namespace" -}}
{{- default .Release.Namespace .Values.dorisOperator.namespaceOverride }}
{{- end }}

{{- define "kube-doris.name" -}}
{{- default .Chart.Name .Values.nameOverride -}}
{{- end }}

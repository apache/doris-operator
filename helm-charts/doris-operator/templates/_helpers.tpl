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
doris operator pod default resource.
*/}}
{{- define "operator.default.resource" }}
    requests:
      cpu: 2
      memory: 4Gi
    limits:
      cpu: 2
      memory: 4Gi
{{- end }}
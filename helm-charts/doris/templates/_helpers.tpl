
{{- define "doriscluster.name" -}}
{{ default .Chart.Name .Values.dorisCluster.name }}
{{- end }}

{{- define "doriscluster.namespace" -}}
{{ default .Release.Namespace .Values.dorisCluster.namespace }}
{{- end }}

{{- define "kube-control.name" -}}
{{- print "doris-operator" }}
{{- end }}
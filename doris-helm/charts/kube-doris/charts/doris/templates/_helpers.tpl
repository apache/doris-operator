
{{- define "doriscluster.name" -}}
{{ default (include "kube-doris.name" .) .Values.dorisCluster.name }}
{{- end }}

{{- define "doriscluster.namespace" -}}
{{ default .Release.Namespace .Values.dorisCluster.namespace }}
{{- end }}

{{- define "kube-doris.name" -}}
{{- default .Chart.Name .Values.nameOverride -}}
{{- end }}

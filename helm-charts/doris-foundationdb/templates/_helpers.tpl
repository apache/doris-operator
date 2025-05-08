{{- define "foundationdb.clusterName"  -}}
{{ default .Chart.Name .Values.name }}
{{- end }}

{{- define "foundationdb.baseImage" }}
{{- if .Values.foundatinondbImage.repository }}
{{- print .Values.foundatinondbImage.repository }}
{{- else }}
{{- print "foundationdb/foundationdb" }}
{{- end }}
{{- end }}


{{- define "foundationdb.tag" }}
{{- if .Values.foundatinondbImage.tag }}
{{- print .Values.foundatinondbImage.tag }}
{{- else }}
{{- print "7.1.38" }}
{{- end }}
{{- end }}

{{- define "foundationdb.sidecar.baseImage" }}
{{- if .Values.foundationdbSidecarImage.repository }}
{{- print .Values.foundationdbSidecarImage.repository }}
{{- else }}
{{- print "foundationdb/foundationdb-kubernetes-sidecar" }}
{{- end }}
{{- end }}

{{- define "foundationdb.sidecar.tag" }}
{{- if .Values.foundationdbSidecarImage.tag }}
{{- print .Values.foundationdbSidecarImage.tag }}
{{- else }}
{{- print "7.1.38-1" }}
{{- end }}
{{- end }}

{{- define "foundationdb.version" }}
{{- if .Values.foundatinondbImage.version }}
{{- print .Values.foundatinondbImage.version }}
{{- else }}
{{- print "7.1.38" }}
{{- end }}
{{- end }}

{{- define "foundationdb.redundancyMode" }}
{{- print "double" }}
{{- end }}

{{- define "foundationdb.processCounts" }}
cluster_controller: 1
log: 2
storage: 2
stateless: -1
{{- end }}

{{- define "foundationdb.resources" }}
requests:
  cpu: 1
  memory: 2Gi
limits:
  cpu: 1
  memory: 4Gi
{{- end }}

{{- define "foundationdb.volumeClaimTemplate" }}
spec:
  resources:
    requests:
      storage: 50G
{{- end }}

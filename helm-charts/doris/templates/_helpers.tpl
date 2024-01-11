{{- define "doriscluster.name" -}}
{{ default .Chart.Name .Values.dorisCluster.name }}
{{- end }}

{{- define "doriscluster.namespace" -}}
{{ default .Release.Namespace .Values.dorisCluster.namespace }}
{{- end }}

{{- define "kube-control.name" -}}
{{- print "doris-operator" }}
{{- end }}

{{/*
doris cluster pod default resource.
*/}}
{{- define "doriscluster.default.resource" }}
    requests:
      cpu: 8
      memory: 16Gi
    limits:
      cpu: 16
      memory: 32Gi
{{- end }}

{{/*
doris cluster pod default configMap resolve file.
*/}}
{{- define "doriscluster.default.feConfig.resolveKey" }}
{{- print "fe.conf" }}
{{- end }}

{{/*
doris cluster pod default configMap resolve file.
*/}}
{{- define "doriscluster.default.beConfig.resolveKey" }}
{{- print "be.conf" }}
{{- end }}

{{/*
doris cluster pod default configMap resolve file.
*/}}
{{- define "doriscluster.default.cnConfig.resolveKey" }}
{{- print "cn.conf" }}
{{- end }}

{{/*
doris cluster pod default configMap resolve file.
*/}}
{{- define "doriscluster.default.brokerConfig.resolveKey" }}
{{- print "apache_hdfs_broker.conf" }}
{{- end }}

{{/*
doris cluster cn pod autoscaler default version.
*/}}
{{- define "doriscluster.default.autoScalerVersion" -}}
{{- print "v2" }}
{{- end -}}



{{/*
doris cluster configMap default name.
*/}}
{{- define "doriscluster.default.configMap.name" -}}
    {{ template "doriscluster.name" . }}-configmap
{{- end -}}


{{/*
doris cluster fe PVC
*/}}
{{- define "doriscluster.fe.pvc" -}}

    {{- if and .Values.feSpec.persistentVolumeClaim.metaPersistentVolume .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/fe/doris-meta
      name: fe-meta
      persistentVolumeClaimSpec:
        {{- if or .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storageClassName .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        storageClassName: {{ default .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storage}}
    {{- end }}
    {{- if and .Values.feSpec.persistentVolumeClaim.logsPersistentVolume .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/fe/log
      name: fe-log
      persistentVolumeClaimSpec:
        {{- if or .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storageClassName}}
        storageClassName: {{ default .Values.feSpec.persistentVolumeClaim.metaPersistentVolume.storageClassName .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.feSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    {{- end }}
{{- end -}}


{{/*
doris cluster be PVC
*/}}
{{- define "doriscluster.be.pvc" -}}

    {{- if and .Values.beSpec.persistentVolumeClaim.dataPersistentVolume .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/be/storage
      name: be-storage
      persistentVolumeClaimSpec:
        {{- if or .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        storageClassName: {{ default .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storage}}
    {{- end }}
    {{- if and .Values.beSpec.persistentVolumeClaim.logsPersistentVolume .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/be/log
      name: be-log
      persistentVolumeClaimSpec:
        {{- if or .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        storageClassName: {{ default .Values.beSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.beSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    {{- end }}
{{- end -}}


{{/*
doris cluster cn PVC
*/}}
{{- define "doriscluster.cn.pvc" -}}

    {{- if and .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/be/storage
      name: cn-storage
      persistentVolumeClaimSpec:
        {{- if or .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        storageClassName: {{ default .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storage}}
    {{- end }}
    {{- if and .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/be/log
      name: cn-log
      persistentVolumeClaimSpec:
        {{- if or .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        storageClassName: {{ default .Values.cnSpec.persistentVolumeClaim.dataPersistentVolume.storageClassName .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.cnSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    {{- end }}
{{- end -}}



{{/*
doris cluster broker PVC
*/}}
{{- define "doriscluster.broker.pvc" -}}
    {{- if and .Values.brokerSpec.persistentVolumeClaim.logsPersistentVolume .Values.brokerSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    - mountPath: /opt/apache-doris/apache_hdfs_broker/log
      name: broker-log
      persistentVolumeClaimSpec:
        {{- if .Values.brokerSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName}}
        storageClassName: {{ .Values.brokerSpec.persistentVolumeClaim.logsPersistentVolume.storageClassName }}
        {{- end }}
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: {{ .Values.brokerSpec.persistentVolumeClaim.logsPersistentVolume.storage}}
    {{- end }}
{{- end -}}
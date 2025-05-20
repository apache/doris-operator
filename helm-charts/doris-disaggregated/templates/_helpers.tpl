{*
 Licensed to the Apache Software Foundation (ASF) under one
 or more contributor license agreements.  See the NOTICE file
 distributed with this work for additional information
 regarding copyright ownership.  The ASF licenses this file
 to you under the Apache License, Version 2.0 (the
 "License"); you may not use this file except in compliance
 with the License.  You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing,
 software distributed under the License is distributed on an
 "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 KIND, either express or implied.  See the License for the
 specific language governing permissions and limitations
 under the License.
*}

{{/*
cluster config
*/}}
{{- define "doris-disaggregated.name" -}}
{{- default .Chart.Name .Values.clusterName | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
ms config part
*/}}
{{- define "ms.fdb.namespace" -}}
{{- default .Release.Namespace .Values.msSpec.fdb.namespace -}}
{{- end }}

{{- define "ms.fdb.configmap.name" -}}
{{ .Values.msSpec.fdb.fdbClusterName }}-config
{{- end }}

{{- define "ms.configmap.name" -}}
{{- print "ms-configmap" }}
{{- end }}

{{- define "ms.configmap.mountpath" -}}
{{- print "/etc/doris" -}}
{{- end }}


{{/*
fe config part
*/}}
{{- define "fe.electionnumber" -}}
{{- default 3 .Values.feSpec.electionNumber -}}
{{- end }}

{{- define "fe.configmap.name" -}}
{{- print "fe-configmap" }}
{{- end }}

{{- define "fe.configmap.mountpath" -}}
{{- print "/etc/doris" -}}
{{- end }}


{{/*
be config part
*/}}
{{- define "be.configmap.name" -}}
{{- print "be-configmap" -}}
{{- end }}

{{- define "be.configmap.mountpath" -}}
{{- print "/etc/doris" -}}
{{- end }}


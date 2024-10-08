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

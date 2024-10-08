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

{{ define "type" }}

<h3 id="{{ anchorIDForType . }}">
    {{- .Name.Name }}
    {{ if eq .Kind "Alias" }}(<code>{{.Underlying}}</code> alias){{ end -}}
</h3>
{{ with (typeReferences .) }}
    <p>
        (<em>Appears on:</em>
        {{- $prev := "" -}}
        {{- range . -}}
            {{- if $prev -}}, {{ end -}}
            {{- $prev = . -}}
            <a href="{{ linkForType . }}">{{ typeDisplayName . }}</a>
        {{- end -}}
        )
    </p>
{{ end }}

<div>
    {{ safe (renderComments .CommentLines) }}
</div>

{{ with (constantsOfType .) }}
<table>
    <thead>
        <tr>
            <th>Value</th>
            <th>Description</th>
        </tr>
    </thead>
    <tbody>
      {{- range . -}}
      <tr>
        {{- /*
            renderComments implicitly creates a <p> element, so we
            add one to the display name as well to make the contents
            of the two cells align evenly.
        */ -}}
        <td><p>{{ typeDisplayName . }}</p></td>
        <td>{{ safe (renderComments .CommentLines) }}</td>
      </tr>
      {{- end -}}
    </tbody>
</table>
{{ end }}

{{ if .Members }}
<table>
    <thead>
        <tr>
            <th>Field</th>
            <th>Description</th>
        </tr>
    </thead>
    <tbody>
        {{ if isExportedType . }}
        <tr>
            <td>
                <code>apiVersion</code><br/>
                string</td>
            <td>
                <code>
                    {{apiGroup .}}
                </code>
            </td>
        </tr>
        <tr>
            <td>
                <code>kind</code><br/>
                string
            </td>
            <td><code>{{.Name.Name}}</code></td>
        </tr>
        {{ end }}
        {{ template "members" .}}
    </tbody>
</table>
{{ end }}

{{ end }}

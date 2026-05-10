{{/*
Expand the name of the chart.
*/}}
{{- define "keyip.name" -}}
{{- default .Chart.Name .Values.global.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
Truncated at 63 chars because some Kubernetes name fields are limited to this.
If release name contains chart name it will be used as a full name.
*/}}
{{- define "keyip.fullname" -}}
{{- if .Values.global.fullnameOverride }}
{{- .Values.global.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.global.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "keyip.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "keyip.labels" -}}
helm.sh/chart: {{ include "keyip.chart" . }}
{{ include "keyip.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "keyip.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keyip.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use for the API server
*/}}
{{- define "keyip.apiserver.serviceAccountName" -}}
{{- if .Values.apiserver.serviceAccountName }}
{{- .Values.apiserver.serviceAccountName }}
{{- else }}
{{- include "keyip.fullname" . }}-apiserver
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use for the worker
*/}}
{{- define "keyip.worker.serviceAccountName" -}}
{{- if .Values.worker.serviceAccountName }}
{{- .Values.worker.serviceAccountName }}
{{- else }}
{{- include "keyip.fullname" . }}-worker
{{- end }}
{{- end }}

{{/*
Default labels for the API server component
*/}}
{{- define "keyip.apiserver.labels" -}}
app.kubernetes.io/component: api
{{ include "keyip.labels" . }}
{{- end }}

{{/*
Default labels for the worker component
*/}}
{{- define "keyip.worker.labels" -}}
app.kubernetes.io/component: worker
{{ include "keyip.labels" . }}
{{- end }}

{{/*
API server selector labels
*/}}
{{- define "keyip.apiserver.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keyip.name" . }}-apiserver
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Worker selector labels
*/}}
{{- define "keyip.worker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "keyip.name" . }}-worker
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Part-of label used consistently across all resources
*/}}
{{- define "keyip.partOf" -}}
app.kubernetes.io/part-of: {{ include "keyip.name" . }}
{{- end }}

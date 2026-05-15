{{/*
Expand the name of the chart.
*/}}
{{- define "blackbox-probe-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "blackbox-probe-controller.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Chart label set
*/}}
{{- define "blackbox-probe-controller.labels" -}}
helm.sh/chart: {{ include "blackbox-probe-controller.chart" . }}
{{ include "blackbox-probe-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: blackbox-probe-controller
{{- end }}

{{- define "blackbox-probe-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "blackbox-probe-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "blackbox-probe-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Operator namespace
*/}}
{{- define "blackbox-probe-controller.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Image reference
*/}}
{{- define "blackbox-probe-controller.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Cluster secret namespace (defaults to release namespace via downward API in pod)
*/}}
{{- define "blackbox-probe-controller.clusterSecretNamespace" -}}
{{- default (include "blackbox-probe-controller.namespace" .) .Values.controller.clusterSecretNamespace }}
{{- end }}

{{- define "blackbox-probe-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-controller-manager" (include "blackbox-probe-controller.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Copyright 2026 k8s-gpu-mcp-server contributors
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "k8s-gpu-mcp-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "k8s-gpu-mcp-server.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "k8s-gpu-mcp-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "k8s-gpu-mcp-server.labels" -}}
helm.sh/chart: {{ include "k8s-gpu-mcp-server.chart" . }}
{{ include "k8s-gpu-mcp-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "k8s-gpu-mcp-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8s-gpu-mcp-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "k8s-gpu-mcp-server.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "k8s-gpu-mcp-server.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the namespace name
*/}}
{{- define "k8s-gpu-mcp-server.namespace" -}}
{{- if .Values.namespace.create }}
{{- .Values.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Validate GPU access configuration.
Ensures only one GPU access method is enabled at a time.
*/}}
{{- define "k8s-gpu-mcp-server.validateGPUConfig" -}}
{{- $enabledCount := 0 }}
{{- $enabledMethods := list }}
{{- if .Values.gpu.runtimeClass.enabled }}
  {{- $enabledCount = add $enabledCount 1 }}
  {{- $enabledMethods = append $enabledMethods "gpu.runtimeClass" }}
{{- end }}
{{- if .Values.gpu.resourceRequest.enabled }}
  {{- $enabledCount = add $enabledCount 1 }}
  {{- $enabledMethods = append $enabledMethods "gpu.resourceRequest" }}
{{- end }}
{{- if .Values.gpu.resourceClaim.enabled }}
  {{- $enabledCount = add $enabledCount 1 }}
  {{- $enabledMethods = append $enabledMethods "gpu.resourceClaim" }}
{{- end }}
{{- if gt $enabledCount 1 }}
  {{- fail (printf "Only one GPU access method can be enabled at a time. Found %d enabled: %s. Disable all but one." $enabledCount (join ", " $enabledMethods)) }}
{{- end }}
{{- if eq $enabledCount 0 }}
  {{- fail "At least one GPU access method must be enabled: gpu.runtimeClass.enabled, gpu.resourceRequest.enabled, or gpu.resourceClaim.enabled" }}
{{- end }}
{{- end }}

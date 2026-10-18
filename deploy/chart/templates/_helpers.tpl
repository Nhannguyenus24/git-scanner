{{/* Common labels for every object. */}}
{{- define "gitscanner.labels" -}}
app.kubernetes.io/name: gitscanner
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/* Selector labels for one service. */}}
{{- define "gitscanner.selectorLabels" -}}
app.kubernetes.io/name: gitscanner
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .name }}
{{- end }}

{{/*
Shared environment. Connection URLs come from the Secret; hostnames resolve
through the Services in external-infra.yaml, so the same values work whether the
backends run in the cluster or on the developer's Docker host (plan.md §13.2).
*/}}
{{- define "gitscanner.env" -}}
- name: ENV
  value: {{ .root.Values.env | quote }}
- name: LOG_LEVEL
  value: {{ .root.Values.logLevel | quote }}
- name: HTTP_ADDR
  value: ":8080"
- name: POSTGRES_URL
  valueFrom:
    secretKeyRef:
      name: {{ .root.Values.secretName }}
      key: POSTGRES_URL
- name: RABBITMQ_URL
  valueFrom:
    secretKeyRef:
      name: {{ .root.Values.secretName }}
      key: RABBITMQ_URL
- name: REDIS_URL
  valueFrom:
    secretKeyRef:
      name: {{ .root.Values.secretName }}
      key: REDIS_URL
- name: KAFKA_BOOTSTRAP_SERVERS
  value: "kafka:{{ .root.Values.infra.kafka.port }}"
{{- end }}

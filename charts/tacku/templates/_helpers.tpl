{{/*
Refusals, in one place so that a missing value stops the render rather than the pod.

The distinction this chart makes is between what is absent and what is wrong. An absent hostname
renders an ingress rule that matches nothing; an absent session key signs everybody out on every
rollout and says so only on stderr; absent issuer settings stop the server from starting at all,
which arrives as a green deploy and a service that is down. None of the three fails loudly on its
own, so all three fail here.
*/}}
{{- define "tacku.require" -}}
{{- if not .value }}{{ fail (printf "tacku: %s is required — %s" .name .because) }}{{ end }}
{{- .value }}
{{- end }}

{{/*
The audience an agent's token must carry, derived from the hostname unless it was set.

Derived rather than defaulted to something: RFC 8707 binds a token to the resource it was issued
for, so this is the address of this server's own MCP endpoint. Two spellings of one address is how
tokens come to verify everywhere except in production.
*/}}
{{- define "tacku.resource" -}}
{{- if .Values.auth.resource }}{{ .Values.auth.resource }}{{ else }}https://{{ .Values.hostname }}/mcp{{ end }}
{{- end }}

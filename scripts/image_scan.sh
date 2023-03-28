#/bin/sh

set -e 

if [ -z $1 ] && [ -z $2 ]; then
    echo "error: image name and arch name are required as arguments. exiting..."
    exit 1
fi

ARCH=$2

# skipping image scan for s390x since trivy doesn't support s390x arch yet
if [ "${ARCH}" == "s390x" ]; then
    exit 0
fi

if [ -n ${DEBUG} ]; then
    set -x
fi

IMAGE=$1
SEVERITIES="HIGH,CRITICAL"
TRIVY_TEMPLATE='{{- $critical := 0 }}{{- $high := 0 }}
{{- println "Target - Severity - ID - Package - Vulnerable Version - Fixed Version" -}}{{ print }}
{{ range . }}
    {{- $target := .Target -}}
    {{ range .Vulnerabilities }}
        {{- if  eq .Severity "CRITICAL" }}{{- $critical = add $critical 1 }}{{- end }}
        {{- if  eq .Severity "HIGH" }}{{- $high = add $high 1 }}{{- end }}
        {{- list $target .Severity .VulnerabilityID .PkgName .InstalledVersion .FixedVersion | join " - " | println -}}
    {{- end -}}
{{ end }}
Vulnerabilities - Critical: {{ $critical }}, High: {{ $high }}{{ println }}'

trivy --quiet image --severity ${SEVERITIES} --no-progress --ignore-unfixed --format template --template "${TRIVY_TEMPLATE}" ${IMAGE}

exit 0

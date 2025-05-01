#/bin/sh

set -e 

if [ -z $1 ]; then
    echo "error: image name is required as argument. exiting..."
    exit 1
fi

# we wont have trivy installed if its an unsupported arch
if [ -z "$(which trivy)" ]; then
    echo "warning: trivy scan being skipped since 'trivy' executable not found in path"
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
VEX_REPORT="rancher.openvex.json"

# Download Rancher's VEX Hub standalone report
curl -fsS -o ${VEX_REPORT} https://raw.githubusercontent.com/rancher/vexhub/refs/heads/main/reports/rancher.openvex.json

trivy --quiet image --severity ${SEVERITIES} --vex ${VEX_REPORT} --no-progress --ignore-unfixed --format template --template "${TRIVY_TEMPLATE}" ${IMAGE}

exit 0

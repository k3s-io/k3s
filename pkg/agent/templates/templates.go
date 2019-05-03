package templates

import (
	"bytes"
	"text/template"

	"github.com/rancher/k3s/pkg/daemons/config"
)

type ContainerdConfig struct {
	NodeConfig        *config.Node
	IsRunningInUserNS bool
}

const ContainerdConfigTemplate = `
[plugins.opt]
path = "{{ .NodeConfig.Containerd.Opt }}"

[plugins.cri]
stream_server_address = "{{ .NodeConfig.AgentConfig.NodeName }}"
stream_server_port = "10010"

{{- if .IsRunningInUserNS }}
disable_cgroup = true
disable_apparmor = true
restrict_oom_score_adj = true
{{ end -}}

{{- if .NodeConfig.AgentConfig.PauseImage }}
sandbox_image = "{{ .NodeConfig.AgentConfig.PauseImage }}"
{{ end -}}

{{- if not .NodeConfig.NoFlannel }}
  [plugins.cri.cni]
    bin_dir = "{{ .NodeConfig.AgentConfig.CNIBinDir }}"
    conf_dir = "{{ .NodeConfig.AgentConfig.CNIConfDir }}"
{{ end -}}
`

func ParseTemplateFromConfig(templateBuffer string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Parse(templateBuffer))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}

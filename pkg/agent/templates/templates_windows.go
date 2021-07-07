// +build windows

package templates

import (
	"bytes"
	"net/url"
	"strings"
	"text/template"
)

const ContainerdConfigTemplate = `
version = 2
root = "{{ replace .NodeConfig.Containerd.Root }}"
state = "{{ replace .NodeConfig.Containerd.State }}"
plugin_dir = ""
disabled_plugins = []
required_plugins = []
oom_score = 0

[grpc]
  address = "{{ deschemify .NodeConfig.Containerd.Address }}"
  tcp_address = ""
  tcp_tls_cert = ""
  tcp_tls_key = ""
  uid = 0
  gid = 0
  max_recv_message_size = 16777216
  max_send_message_size = 16777216

[ttrpc]
  address = ""
  uid = 0
  gid = 0

[debug]
  address = ""
  uid = 0
  gid = 0
  level = ""

[metrics]
  address = ""
  grpc_histogram = false

[cgroup]
  path = ""

[timeouts]
  "io.containerd.timeout.shim.cleanup" = "5s"
  "io.containerd.timeout.shim.load" = "5s"
  "io.containerd.timeout.shim.shutdown" = "3s"
  "io.containerd.timeout.task.state" = "2s"

[plugins]
  [plugins."io.containerd.gc.v1.scheduler"]
    pause_threshold = 0.02
    deletion_threshold = 0
    mutation_threshold = 100
    schedule_delay = "0s"
    startup_delay = "100ms"
  [plugins."io.containerd.grpc.v1.cri"]
    disable_tcp_service = true
    stream_server_address = "127.0.0.1"
    stream_server_port = "0"
    stream_idle_timeout = "4h0m0s"
    enable_selinux = false
    selinux_category_range = 0
    sandbox_image = "{{ .NodeConfig.AgentConfig.PauseImage }}"
    stats_collect_period = 10
    systemd_cgroup = false
    enable_tls_streaming = false
    max_container_log_line_size = 16384
    disable_cgroup = false
    disable_apparmor = false
    restrict_oom_score_adj = false
    max_concurrent_downloads = 3
    disable_proc_mount = false
    unset_seccomp_profile = ""
    tolerate_missing_hugetlb_controller = false
    disable_hugetlb_controller = false
    ignore_image_defined_volumes = false
    [plugins."io.containerd.grpc.v1.cri".containerd]
      snapshotter = "windows"
      default_runtime_name = "runhcs-wcow-process"
      no_pivot = false
      disable_snapshot_annotations = false
      discard_unpacked_layers = false
      [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
        runtime_type = ""
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
        runtime_type = ""
        runtime_engine = ""
        runtime_root = ""
        privileged_without_host_devices = false
        base_runtime_spec = ""
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runhcs-wcow-process]
          runtime_type = "io.containerd.runhcs.v1"
          runtime_engine = ""
          runtime_root = ""
          privileged_without_host_devices = false
          base_runtime_spec = ""
    [plugins."io.containerd.grpc.v1.cri".cni]
      bin_dir = "{{ replace .NodeConfig.AgentConfig.CNIBinDir }}"
      conf_dir = "{{ replace .NodeConfig.AgentConfig.CNIConfDir }}"
      max_conf_num = 1
      conf_template = ""
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = ""
      {{range $k, $v := .PrivateRegistryConfig.Configs }}
      [plugins."io.containerd.grpc.v1.cri".registry.auths]
      {{ if $v.Auth }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs.auth."{{$k}}"]
          {{ if $v.Auth.Username }}username = {{ printf "%q" $v.Auth.Username }}{{end}}
          {{ if $v.Auth.Password }}password = {{ printf "%q" $v.Auth.Password }}{{end}}
          {{ if $v.Auth.Auth }}auth = {{ printf "%q" $v.Auth.Auth }}{{end}}
          {{ if $v.Auth.IdentityToken }}identitytoken = {{ printf "%q" $v.Auth.IdentityToken }}{{end}}
      {{end}}
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
      {{ if $v.TLS }}
        [plugins."io.containerd.grpc.v1.cri".registry.configs.tls."{{$k}}"]
          {{ if $v.TLS.CAFile }}ca_file = "{{ $v.TLS.CAFile }}"{{end}}
          {{ if $v.TLS.CertFile }}cert_file = "{{ $v.TLS.CertFile }}"{{end}}
          {{ if $v.TLS.KeyFile }}key_file = "{{ $v.TLS.KeyFile }}"{{end}}
          {{ if $v.TLS.InsecureSkipVerify }}insecure_skip_verify = true{{end}}
      {{end}}
      {{end}}
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
      {{ if .PrivateRegistryConfig.Mirrors }}
      {{range $k, $v := .PrivateRegistryConfig.Mirrors }}
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{$k}}"]
	        endpoint = [{{range $i, $j := $v.Endpoints}}{{if $i}}, {{end}}{{printf "%q" .}}{{end}}]
	    {{if $v.Rewrites}}
	      [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{$k}}".rewrite]
	      {{range $pattern, $replace := $v.Rewrites}}
	        "{{$pattern}}" = "{{$replace}}"
	      {{end}}
	    {{end}}
      {{end}}
    {{end}}
    [plugins."io.containerd.grpc.v1.cri".image_decryption]
      key_model = ""
    [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
      tls_cert_file = ""
      tls_key_file = ""
  [plugins."io.containerd.internal.v1.opt"]
    path = "{{ replace .NodeConfig.Containerd.Opt }}"
  [plugins."io.containerd.internal.v1.restart"]
    interval = "10s"
  [plugins."io.containerd.metadata.v1.bolt"]
    content_sharing_policy = "shared"
  [plugins."io.containerd.runtime.v2.task"]
    platforms = ["windows/amd64", "linux/amd64"]
  [plugins."io.containerd.service.v1.diff-service"]
    default = ["windows", "windows-lcow"]
`

func ParseTemplateFromConfig(templateBuffer string, config interface{}) (string, error) {
	out := new(bytes.Buffer)
	funcs := template.FuncMap{
		"replace": func(s string) string {
			return strings.ReplaceAll(s, "\\", "\\\\")
		},
		"deschemify": func(s string) string {
			if strings.HasPrefix(s, "npipe:") {
				u, err := url.Parse(s)
				if err != nil {
					return ""
				}
				return u.Path
			}
			return s
		},
	}
	t := template.Must(template.New("compiled_template").Funcs(funcs).Parse(templateBuffer))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}

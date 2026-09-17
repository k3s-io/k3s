package util

import (
	"strings"

	docker "github.com/distribution/reference"
)

// DefaultSystemRegistry is where K3s publishes the images it deploys itself.
// The bundled manifests name it through %{SYSTEM_IMAGE_REGISTRY}%,
// a cluster started without --system-default-registry still pulls k3s images from here.
// This is intentional to not be default for config.Control.SystemDefaultRegistry, which stays
// empty unless the config sets it. If the config does not set it, the user workflow images will be pulled from docker hub.
const DefaultSystemRegistry = "ghcr.io"

func SystemRegistryOrDefault(systemDefaultRegistry string) string {
	if host := strings.TrimSuffix(systemDefaultRegistry, "/"); host != "" {
		return host
	}
	return DefaultSystemRegistry
}

func ImageWithRegistry(registry, ref string) string {
	registry = strings.TrimSuffix(registry, "/")
	if registry == "" || ref == "" {
		return ref
	}
	if domain, ok := explicitRegistryDomain(ref); ok {
		if domain == registry {
			return ref
		}
		return registry + strings.TrimPrefix(ref, domain)
	}
	if strings.HasPrefix(ref, registry+"/") {
		return ref
	}
	return registry + "/" + ref
}

func explicitRegistryDomain(ref string) (string, bool) {
	named, err := docker.ParseNormalizedNamed(ref)
	if err != nil {
		return "", false
	}
	domain := docker.Domain(named)
	return domain, strings.HasPrefix(ref, domain+"/")
}

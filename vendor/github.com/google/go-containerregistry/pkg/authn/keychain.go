// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authn

import (
	"os"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/name"
)

// Resource represents a registry or repository that can be authenticated against.
type Resource interface {
	// String returns the full string representation of the target, e.g.
	// gcr.io/my-project or just gcr.io.
	String() string

	// RegistryStr returns just the registry portion of the target, e.g. for
	// gcr.io/my-project, this should just return gcr.io. This is needed to
	// pull out an appropriate hostname.
	RegistryStr() string
}

// Keychain is an interface for resolving an image reference to a credential.
type Keychain interface {
	// Resolve looks up the most appropriate credential for the specified target.
	Resolve(Resource) (Authenticator, error)
}

// defaultKeychain implements Keychain with the semantics of the standard Docker
// credential keychain.
type defaultKeychain struct {
	mu sync.Mutex
}

var (
	// DefaultKeychain implements Keychain by interpreting the docker config file.
	DefaultKeychain Keychain = &defaultKeychain{}
)

const (
	// DefaultAuthKey is the key used for dockerhub in config files, which
	// is hardcoded for historical reasons.
	DefaultAuthKey = "https://" + name.DefaultRegistry + "/v1/"
)

// Resolve implements Keychain.
func (dk *defaultKeychain) Resolve(target Resource) (Authenticator, error) {
	dk.mu.Lock()
	defer dk.mu.Unlock()
	cf, err := config.Load(os.Getenv("DOCKER_CONFIG"))
	if err != nil {
		return nil, err
	}

	// See:
	// https://github.com/google/ko/issues/90
	// https://github.com/moby/moby/blob/fc01c2b481097a6057bec3cd1ab2d7b4488c50c4/registry/config.go#L397-L404
	key := target.RegistryStr()
	if key == name.DefaultRegistry {
		key = DefaultAuthKey
	}

	cfg, err := cf.GetAuthConfig(key)
	if err != nil {
		return nil, err
	}

	empty := types.AuthConfig{}
	if cfg == empty {
		return Anonymous, nil
	}
	return FromConfig(AuthConfig{
		Username:      cfg.Username,
		Password:      cfg.Password,
		Auth:          cfg.Auth,
		IdentityToken: cfg.IdentityToken,
		RegistryToken: cfg.RegistryToken,
	}), nil
}

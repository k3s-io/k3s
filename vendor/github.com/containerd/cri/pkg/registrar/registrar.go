/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registrar

import (
	"sync"

	"github.com/pkg/errors"
)

// Registrar stores one-to-one name<->key mappings.
// Names and keys must be unique.
// Registrar is safe for concurrent access.
type Registrar struct {
	lock      sync.Mutex
	nameToKey map[string]string
	keyToName map[string]string
}

// NewRegistrar creates a new Registrar with the empty indexes.
func NewRegistrar() *Registrar {
	return &Registrar{
		nameToKey: make(map[string]string),
		keyToName: make(map[string]string),
	}
}

// Reserve registers a name<->key mapping, name or key must not
// be empty.
// Reserve is idempotent.
// Attempting to reserve a conflict key<->name mapping results
// in an error.
// A name<->key reservation is globally unique.
func (r *Registrar) Reserve(name, key string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if name == "" || key == "" {
		return errors.Errorf("invalid name %q or key %q", name, key)
	}

	if k, exists := r.nameToKey[name]; exists {
		if k != key {
			return errors.Errorf("name %q is reserved for %q", name, k)
		}
		return nil
	}

	if n, exists := r.keyToName[key]; exists {
		if n != name {
			return errors.Errorf("key %q is reserved for %q", key, n)
		}
		return nil
	}

	r.nameToKey[name] = key
	r.keyToName[key] = name
	return nil
}

// ReleaseByName releases the reserved name<->key mapping by name.
// Once released, the name and the key can be reserved again.
func (r *Registrar) ReleaseByName(name string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	key, exists := r.nameToKey[name]
	if !exists {
		return
	}

	delete(r.nameToKey, name)
	delete(r.keyToName, key)
}

// ReleaseByKey release the reserved name<->key mapping by key.
func (r *Registrar) ReleaseByKey(key string) {
	r.lock.Lock()
	defer r.lock.Unlock()

	name, exists := r.keyToName[key]
	if !exists {
		return
	}

	delete(r.nameToKey, name)
	delete(r.keyToName, key)
}

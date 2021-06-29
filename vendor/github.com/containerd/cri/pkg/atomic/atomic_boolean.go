/*
   Copyright The containerd Authors.

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

package atomic

import "sync/atomic"

// Bool is an atomic Boolean,
// Its methods are all atomic, thus safe to be called by
// multiple goroutines simultaneously.
type Bool interface {
	Set()
	Unset()
	IsSet() bool
}

// NewBool creates an Bool with given default value
func NewBool(ok bool) Bool {
	ab := new(atomicBool)
	if ok {
		ab.Set()
	}
	return ab
}

type atomicBool int32

// Set sets the Boolean to true
func (ab *atomicBool) Set() {
	atomic.StoreInt32((*int32)(ab), 1)
}

// Unset sets the Boolean to false
func (ab *atomicBool) Unset() {
	atomic.StoreInt32((*int32)(ab), 0)
}

// IsSet returns whether the Boolean is true
func (ab *atomicBool) IsSet() bool {
	return atomic.LoadInt32((*int32)(ab)) == 1
}

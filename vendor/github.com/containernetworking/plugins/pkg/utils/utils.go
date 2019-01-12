// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"crypto/sha512"
	"fmt"
)

const (
	maxChainLength = 28
	chainPrefix    = "CNI-"
	prefixLength   = len(chainPrefix)
)

// Generates a chain name to be used with iptables.
// Ensures that the generated chain name is exactly
// maxChainLength chars in length
func FormatChainName(name string, id string) string {
	chainBytes := sha512.Sum512([]byte(name + id))
	chain := fmt.Sprintf("%s%x", chainPrefix, chainBytes)
	return chain[:maxChainLength]
}

// FormatComment returns a comment used for easier
// rule identification within iptables.
func FormatComment(name string, id string) string {
	return fmt.Sprintf("name: %q id: %q", name, id)
}

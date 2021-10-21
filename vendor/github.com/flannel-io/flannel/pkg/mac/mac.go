// Copyright 2021 flannel authors
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

package mac

import (
	"crypto/rand"
	"fmt"
	"net"
)

// NewHardwareAddr generates a new random hardware (MAC) address, local and
// unicast.
func NewHardwareAddr() (net.HardwareAddr, error) {
	hardwareAddr := make(net.HardwareAddr, 6)
	if _, err := rand.Read(hardwareAddr); err != nil {
		return nil, fmt.Errorf("could not generate random MAC address: %w", err)
	}

	// Ensure that address is locally administered and unicast.
	hardwareAddr[0] = (hardwareAddr[0] & 0xfe) | 0x02

	return hardwareAddr, nil
}

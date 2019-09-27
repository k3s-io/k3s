// Copyright 2019 CNI authors
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

// Buildversion is a destination for the linker trickery so we can auto
// set the build-version
package buildversion

import "fmt"

// This is overridden in the linker script
var BuildVersion = "version unknown"

func BuildString(pluginName string) string {
	return fmt.Sprintf("CNI %s plugin %s", pluginName, BuildVersion)
}

// +build ignore

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

package continuity

import "os"

var (
	devNullResource = resource{
		kind:  chardev,
		path:  "/dev/null",
		major: 3,
		minor: 2,
		mode:  0666 | os.ModeDevice | os.ModeCharDevice,
	}

	devZeroResource = resource{
		kind:  chardev,
		path:  "/dev/zero",
		major: 3,
		minor: 3,
		mode:  0666 | os.ModeDevice | os.ModeCharDevice,
	}
)

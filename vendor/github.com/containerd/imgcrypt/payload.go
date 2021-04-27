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

package imgcrypt

import (
	"github.com/containerd/typeurl"
	encconfig "github.com/containers/ocicrypt/config"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	PayloadURI = "io.containerd.ocicrypt.v1.Payload"
)

var PayloadToolIDs = []string{
	"io.containerd.ocicrypt.decoder.v1.tar",
	"io.containerd.ocicrypt.decoder.v1.tar.gzip",
}

func init() {
	typeurl.Register(&Payload{}, PayloadURI)
}

// Payload holds data that the external layer decryption tool
// needs for decrypting a layer
type Payload struct {
	DecryptConfig encconfig.DecryptConfig
	Descriptor    ocispec.Descriptor
}

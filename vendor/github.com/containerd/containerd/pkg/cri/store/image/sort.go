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

package image

import (
	"sort"

	"github.com/containerd/containerd/reference/docker"
)

// sortReferences sorts references by refRank then string comparison
func sortReferences(references []string) []string {
	var prefs []docker.Reference
	var bad []string

	for _, ref := range references {
		pref, err := docker.ParseAnyReference(ref)
		if err != nil {
			bad = append(bad, ref)
		} else {
			prefs = append(prefs, pref)
		}
	}
	sort.Slice(prefs, func(a, b int) bool {
		ar := refRank(prefs[a])
		br := refRank(prefs[b])
		if ar == br {
			return prefs[a].String() < prefs[b].String()
		}
		return ar < br
	})
	sort.Strings(bad)
	var refs []string
	for _, pref := range prefs {
		refs = append(refs, pref.String())
	}
	return append(refs, bad...)
}

// refRank ranks precedence for reference type, preferring higher information references
// 1. Name + Tag + Digest
// 2. Name + Tag
// 3. Name + Digest
// 4. Name
// 5. Digest
// 6. Parse error
func refRank(ref docker.Reference) uint8 {
	if _, ok := ref.(docker.Named); ok {
		if _, ok = ref.(docker.Tagged); ok {
			if _, ok = ref.(docker.Digested); ok {
				return 1
			}
			return 2
		}
		if _, ok = ref.(docker.Digested); ok {
			return 3
		}
		return 4
	}
	return 5
}

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

package hostlocal

import (
	"bufio"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
)

// parseResolvConf parses an existing resolv.conf in to a DNS struct
func parseResolvConf(filename string) (*types.DNS, error) {
	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	dns := types.DNS{}
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip comments, empty lines
		if len(line) == 0 || line[0] == '#' || line[0] == ';' {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			dns.Nameservers = append(dns.Nameservers, fields[1])
		case "domain":
			dns.Domain = fields[1]
		case "search":
			dns.Search = append(dns.Search, fields[1:]...)
		case "options":
			dns.Options = append(dns.Options, fields[1:]...)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &dns, nil
}

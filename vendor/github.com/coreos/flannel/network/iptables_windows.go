// Copyright 2015 flannel authors
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

package network

import (
	"github.com/coreos/flannel/pkg/ip"
	"github.com/coreos/flannel/subnet"
)

type IPTables interface {
	AppendUnique(table string, chain string, rulespec ...string) error
	Delete(table string, chain string, rulespec ...string) error
	Exists(table string, chain string, rulespec ...string) (bool, error)
}

type IPTablesRule struct {
	table    string
	chain    string
	rulespec []string
}

func MasqRules(ipn ip.IP4Net, lease *subnet.Lease) []IPTablesRule {
	return nil
}

func ForwardRules(flannelNetwork string) []IPTablesRule {
	return nil
}

func SetupAndEnsureIPTables(rules []IPTablesRule, resyncPeriod int) {

}

func DeleteIPTables(rules []IPTablesRule) error {
	return nil
}

func teardownIPTables(ipt IPTables, rules []IPTablesRule) {
}

// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import (
	"strconv"
	"time"
)

func init() {
	ToStringMap["string"] = func(value interface{}) (string, bool) {
		return value.(string), true
	}
	ToStringMap["bool"] = func(value interface{}) (string, bool) {
		return strconv.FormatBool(value.(bool)), true
	}
	ToStringMap["int"] = func(value interface{}) (string, bool) {
		return strconv.Itoa(value.(int)), true
	}
	ToStringMap["int64"] = func(value interface{}) (string, bool) {
		return strconv.FormatInt(value.(int64), 10), true
	}
	ToStringMap["uint"] = func(value interface{}) (string, bool) {
		return strconv.FormatUint(uint64(value.(uint)), 10), true
	}
	ToStringMap["uint64"] = func(value interface{}) (string, bool) {
		return strconv.FormatUint(value.(uint64), 10), true
	}
	ToStringMap["float64"] = func(value interface{}) (string, bool) {
		return strconv.FormatFloat(value.(float64), 'g', -1, 64), true
	}
	ToStringMap["time.Duration"] = func(value interface{}) (string, bool) {
		return value.(time.Duration).String(), true
	}
	ToStringMap["time.Time"] = func(value interface{}) (string, bool) {
		if len(TimeLayouts) > 0 {
			return value.(time.Time).Format(TimeLayouts[0]), true
		}
		return "", false
	}
}

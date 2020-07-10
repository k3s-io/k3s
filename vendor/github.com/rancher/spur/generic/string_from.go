// Copyright 2020 Rancher Labs, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import (
	"strconv"
	"time"
)

func init() {
	FromStringMap["string"] = func(s string) (interface{}, error) {
		return s, nil
	}
	FromStringMap["bool"] = func(s string) (interface{}, error) {
		if s == "" {
			s = "false"
		}
		v, err := strconv.ParseBool(s)
		return bool(v), err
	}
	FromStringMap["int"] = func(s string) (interface{}, error) {
		v, err := strconv.ParseInt(s, 0, strconv.IntSize)
		return int(v), err
	}
	FromStringMap["int64"] = func(s string) (interface{}, error) {
		v, err := strconv.ParseInt(s, 0, 64)
		return int64(v), err
	}
	FromStringMap["uint"] = func(s string) (interface{}, error) {
		v, err := strconv.ParseUint(s, 0, strconv.IntSize)
		return uint(v), err
	}
	FromStringMap["uint64"] = func(s string) (interface{}, error) {
		v, err := strconv.ParseUint(s, 0, 64)
		return uint64(v), err
	}
	FromStringMap["float64"] = func(s string) (interface{}, error) {
		v, err := strconv.ParseFloat(s, 64)
		return float64(v), err
	}
	FromStringMap["time.Duration"] = func(s string) (interface{}, error) {
		if v, err := time.ParseDuration(s); err == nil {
			return time.Duration(v), nil
		}
		return nil, errParse
	}
	FromStringMap["time.Time"] = func(s string) (interface{}, error) {
		for _, layout := range TimeLayouts {
			if v, err := time.Parse(layout, s); err == nil {
				return time.Time(v), nil
			}
		}
		return nil, errParse
	}
}

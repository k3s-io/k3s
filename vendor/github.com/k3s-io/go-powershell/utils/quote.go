// Copyright (c) 2017 Gorillalabs. All rights reserved.

package utils

import "strings"

func QuoteArg(s string) string {
	return "'" + strings.Replace(s, "'", "\"", -1) + "'"
}

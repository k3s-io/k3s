// Copyright (c) 2017 Gorillalabs. All rights reserved.

package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func CreateRandomString(bytes int) string {
	c := bytes
	b := make([]byte, c)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)
}

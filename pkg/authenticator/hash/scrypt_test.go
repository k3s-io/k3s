package hash

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var hasher = NewSCrypt()

func TestBasicHash(t *testing.T) {
	secretKey := "hello world"
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)

	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, "goodbye"))
}

func TestLongKey(t *testing.T) {
	secretKey := strings.Repeat("A", 720)
	hash, err := hasher.CreateHash(secretKey)
	assert.Nil(t, err)
	assert.NotNil(t, hash)

	assert.Nil(t, hasher.VerifyHash(hash, secretKey))
	assert.NotNil(t, hasher.VerifyHash(hash, secretKey+":wrong!"))
}

package hash

// Hasher is a generic interface for hashing algorithms
type Hasher interface {
	// CreateHash will return a hashed version of the secretKey, or an error
	CreateHash(secretKey string) (string, error)
	// VerifyHash will compare a secretKey and a hash, and return nil if they match
	VerifyHash(hash, secretKey string) error
}

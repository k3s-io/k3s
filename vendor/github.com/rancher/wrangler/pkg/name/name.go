package name

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func GuessPluralName(name string) string {
	if name == "" {
		return name
	}

	if strings.EqualFold(name, "Endpoints") {
		return name
	}

	if suffix(name, "s") || suffix(name, "ch") || suffix(name, "x") || suffix(name, "sh") {
		return name + "es"
	}

	if suffix(name, "f") || suffix(name, "fe") {
		return name + "ves"
	}

	if suffix(name, "y") && len(name) > 2 && !strings.ContainsAny(name[len(name)-2:len(name)-1], "[aeiou]") {
		return name[0:len(name)-1] + "ies"
	}

	return name + "s"
}

func suffix(str, end string) bool {
	return strings.HasSuffix(str, end)
}

func Limit(s string, count int) string {
	if len(s) < count {
		return s
	}
	return fmt.Sprintf("%s-%s", s[:count-6], Hex(s, 5))
}

func Hex(s string, length int) string {
	h := md5.Sum([]byte(s))
	d := hex.EncodeToString(h[:])
	return d[:length]
}

func SafeConcatName(name ...string) string {
	fullPath := strings.Join(name, "-")
	if len(fullPath) > 63 {
		digest := sha256.Sum256([]byte(fullPath))
		return fullPath[0:57] + "-" + hex.EncodeToString(digest[0:])[0:5]
	}
	return fullPath
}

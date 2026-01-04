package nodepassword

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const migrateNumNodes = 10
const createNumNodes = 3

//revive:disable:deep-exit

func Test_UnitAsserts(t *testing.T) {
	assertEqual(t, 1, 1)
	assertNotEqual(t, 1, 0)
}

func Test_PasswordError(t *testing.T) {
	err := &passwordError{node: "test", err: errors.New("inner error")}
	assertEqual(t, errors.Is(err, ErrVerifyFailed), true)
	assertEqual(t, errors.Is(err, errors.New("different error")), false)
	assertNotEqual(t, errors.Unwrap(err), nil)
}

func TestIsAlreadyExists(t *testing.T) {
	assertEqual(t, isAlreadyExists(errorAlreadyExists()), true)
	assertEqual(t, isAlreadyExists(fmt.Errorf("wrapped: %w", errorAlreadyExists())), true)
	assertEqual(t, isAlreadyExists(errorNotFound()), false)
	assertEqual(t, isAlreadyExists(nil), false)
}

// --------------------------
// utility functions

func assertEqual(t *testing.T, a any, b any) {
	if a != b {
		t.Fatalf("[ %v != %v ]", a, b)
	}
}

func assertNotEqual(t *testing.T, a any, b any) {
	if a == b {
		t.Fatalf("[ %v == %v ]", a, b)
	}
}

func generateNodePasswordFile(migrateNumNodes int) string {
	tempFile, err := os.CreateTemp("", "node-password-test.*")
	if err != nil {
		log.Fatal(err)
	}
	tempFile.Close()

	var passwordEntries string
	for i := 1; i <= migrateNumNodes; i++ {
		passwordEntries += fmt.Sprintf("node%d,node%d\n", i, i)
	}
	if err := os.WriteFile(tempFile.Name(), []byte(passwordEntries), 0600); err != nil {
		log.Fatal(err)
	}

	return tempFile.Name()
}

func errorNotFound() error {
	return apierrors.NewNotFound(schema.GroupResource{}, "not-found")
}

func errorAlreadyExists() error {
	return apierrors.NewAlreadyExists(schema.GroupResource{}, "already-exists")
}

func logMemUsage(t *testing.T) {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	t.Logf("Memory Usage:  Alloc=%d MB,  Sys=%d MB,  NumGC=%d",
		toMB(stats.Alloc), toMB(stats.Sys), stats.NumGC)
}

func toMB(bytes uint64) uint64 {
	return bytes / (1024 * 1024)
}

package nodepassword

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"testing"

	"github.com/k3s-io/k3s/tests/mock"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const migrateNumNodes = 10
const createNumNodes = 3

func Test_UnitAsserts(t *testing.T) {
	assertEqual(t, 1, 1)
	assertNotEqual(t, 1, 0)
}

func Test_UnitEnsureDelete(t *testing.T) {
	logMemUsage(t)

	v1Mock := mock.NewV1(gomock.NewController(t))

	secretClient := v1Mock.SecretMock
	secretCache := v1Mock.SecretCache
	secretStore := &mock.SecretStore{}

	// Set up expected call counts for tests
	// Expect to see 2 creates, any number of cache gets, and 2 deletes.
	secretClient.EXPECT().Create(gomock.Any()).Times(2).DoAndReturn(secretStore.Create)
	secretClient.EXPECT().Delete(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).DoAndReturn(secretStore.Delete)
	secretClient.EXPECT().Cache().AnyTimes().Return(secretCache)
	secretCache.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(secretStore.Get)

	// Run tests
	assertEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertNotEqual(t, Ensure(secretClient, "node1", "Goodbye World"), nil)

	assertEqual(t, Delete(secretClient, "node1"), nil)
	assertNotEqual(t, Delete(secretClient, "node1"), nil)

	assertEqual(t, Ensure(secretClient, "node1", "Hello Universe"), nil)
	assertNotEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "Hello Universe"), nil)

	logMemUsage(t)
}

func Test_UnitMigrateFile(t *testing.T) {
	nodePasswordFile := generateNodePasswordFile(migrateNumNodes)
	defer os.Remove(nodePasswordFile)

	v1Mock := mock.NewV1(gomock.NewController(t))

	secretClient := v1Mock.SecretMock
	secretCache := v1Mock.SecretCache
	secretStore := &mock.SecretStore{}

	nodeClient := v1Mock.NodeMock
	nodeCache := v1Mock.NodeCache
	nodeStore := &mock.NodeStore{}

	// Set up expected call counts for tests
	// Expect to see 1 node list, any number of cache gets, and however many
	// creates as we are migrating.
	secretClient.EXPECT().Create(gomock.Any()).Times(migrateNumNodes).DoAndReturn(secretStore.Create)
	secretClient.EXPECT().Cache().AnyTimes().Return(secretCache)
	secretCache.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(secretStore.Get)
	nodeClient.EXPECT().Cache().AnyTimes().Return(nodeCache)
	nodeCache.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(nodeStore.List)

	// Run tests
	logMemUsage(t)
	if err := MigrateFile(secretClient, nodeClient, nodePasswordFile); err != nil {
		t.Fatal(err)
	}
	logMemUsage(t)

	assertNotEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "node1"), nil)
}

func Test_UnitMigrateFileNodes(t *testing.T) {
	nodePasswordFile := generateNodePasswordFile(migrateNumNodes)
	defer os.Remove(nodePasswordFile)

	v1Mock := mock.NewV1(gomock.NewController(t))

	secretClient := v1Mock.SecretMock
	secretCache := v1Mock.SecretCache
	secretStore := &mock.SecretStore{}

	nodeClient := v1Mock.NodeMock
	nodeCache := v1Mock.NodeCache
	nodeStore := &mock.NodeStore{}

	for i := 0; i < createNumNodes; i++ {
		if _, err := nodeStore.Create(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node%d", i+1)}}); err != nil {
			t.Fatal(err)
		}
	}

	// Set up expected call counts for tests
	// Expect to see 1 node list, any number of cache gets, and however many
	// creates as we are migrating - plus an extra new node at the end.
	secretClient.EXPECT().Create(gomock.Any()).Times(migrateNumNodes + 1).DoAndReturn(secretStore.Create)
	secretClient.EXPECT().Cache().AnyTimes().Return(secretCache)
	secretCache.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(secretStore.Get)
	nodeClient.EXPECT().Cache().AnyTimes().Return(nodeCache)
	nodeCache.EXPECT().List(gomock.Any()).Times(1).DoAndReturn(nodeStore.List)

	// Run tests
	logMemUsage(t)
	if err := MigrateFile(secretClient, nodeClient, nodePasswordFile); err != nil {
		t.Fatal(err)
	}
	logMemUsage(t)

	if nodes, err := nodeStore.List(labels.Everything()); err != nil {
		t.Fatal(err)
	} else {
		for _, node := range nodes {
			assertNotEqual(t, Ensure(secretClient, node.Name, "wrong-password"), nil)
			assertEqual(t, Ensure(secretClient, node.Name, node.Name), nil)
		}
	}

	newNode := fmt.Sprintf("node%d", migrateNumNodes+1)
	assertEqual(t, Ensure(secretClient, newNode, "new-password"), nil)
	assertNotEqual(t, Ensure(secretClient, newNode, "wrong-password"), nil)
}

func Test_PasswordError(t *testing.T) {
	err := &passwordError{node: "test", err: fmt.Errorf("inner error")}
	assertEqual(t, errors.Is(err, ErrVerifyFailed), true)
	assertEqual(t, errors.Is(err, fmt.Errorf("different error")), false)
	assertNotEqual(t, errors.Unwrap(err), nil)
}

// --------------------------
// utility functions

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Fatalf("[ %v != %v ]", a, b)
	}
}

func assertNotEqual(t *testing.T, a interface{}, b interface{}) {
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

package nodepassword

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

const migrateNumNodes = 10
const createNumNodes = 3

func TestAsserts(t *testing.T) {
	assertEqual(t, 1, 1)
	assertNotEqual(t, 1, 0)
}

func TestEnsureDelete(t *testing.T) {
	logMemUsage(t)

	secretClient := &mockSecretClient{}
	assertEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertNotEqual(t, Ensure(secretClient, "node1", "Goodbye World"), nil)
	assertEqual(t, secretClient.created, 1)

	assertEqual(t, Delete(secretClient, "node1"), nil)
	assertNotEqual(t, Delete(secretClient, "node1"), nil)
	assertEqual(t, secretClient.deleted, 1)

	assertEqual(t, Ensure(secretClient, "node1", "Hello Universe"), nil)
	assertNotEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "Hello Universe"), nil)
	assertEqual(t, secretClient.created, 2)

	logMemUsage(t)
}

func TestMigrateFile(t *testing.T) {
	nodePasswordFile := generateNodePasswordFile(migrateNumNodes)
	defer os.Remove(nodePasswordFile)

	secretClient := &mockSecretClient{}
	nodeClient := &mockNodeClient{}

	logMemUsage(t)
	if err := MigrateFile(secretClient, nodeClient, nodePasswordFile); err != nil {
		log.Fatal(err)
	}
	logMemUsage(t)

	assertEqual(t, secretClient.created, migrateNumNodes)
	assertNotEqual(t, Ensure(secretClient, "node1", "Hello World"), nil)
	assertEqual(t, Ensure(secretClient, "node1", "node1"), nil)
}

func TestMigrateFileNodes(t *testing.T) {
	nodePasswordFile := generateNodePasswordFile(migrateNumNodes)
	defer os.Remove(nodePasswordFile)

	secretClient := &mockSecretClient{}
	nodeClient := &mockNodeClient{}
	nodeClient.nodes = make([]v1.Node, createNumNodes, createNumNodes)
	for i := range nodeClient.nodes {
		nodeClient.nodes[i].Name = fmt.Sprintf("node%d", i+1)
	}

	logMemUsage(t)
	if err := MigrateFile(secretClient, nodeClient, nodePasswordFile); err != nil {
		log.Fatal(err)
	}
	logMemUsage(t)

	assertEqual(t, secretClient.created, createNumNodes)
	for _, node := range nodeClient.nodes {
		assertNotEqual(t, Ensure(secretClient, node.Name, "wrong-password"), nil)
		assertEqual(t, Ensure(secretClient, node.Name, node.Name), nil)
	}
	newNode := fmt.Sprintf("node%d", createNumNodes+1)
	assertEqual(t, Ensure(secretClient, newNode, "new-password"), nil)
	assertNotEqual(t, Ensure(secretClient, newNode, "wrong-password"), nil)
}

// --------------------------

// mock secret client interface

type mockSecretClient struct {
	entries map[string]map[string]v1.Secret
	created int
	deleted int
}

func (m *mockSecretClient) Create(secret *v1.Secret) (*v1.Secret, error) {
	if m.entries == nil {
		m.entries = map[string]map[string]v1.Secret{}
	}
	if _, ok := m.entries[secret.Namespace]; !ok {
		m.entries[secret.Namespace] = map[string]v1.Secret{}
	}
	if _, ok := m.entries[secret.Namespace][secret.Name]; ok {
		return nil, errorAlreadyExists()
	}
	m.created++
	m.entries[secret.Namespace][secret.Name] = *secret
	return secret, nil
}

func (m *mockSecretClient) Update(secret *v1.Secret) (*v1.Secret, error) {
	return nil, errorNotImplemented()
}

func (m *mockSecretClient) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if m.entries == nil {
		return errorNotFound()
	}
	if _, ok := m.entries[namespace]; !ok {
		return errorNotFound()
	}
	if _, ok := m.entries[namespace][name]; !ok {
		return errorNotFound()
	}
	m.deleted++
	delete(m.entries[namespace], name)
	return nil
}

func (m *mockSecretClient) Get(namespace, name string, options metav1.GetOptions) (*v1.Secret, error) {
	if m.entries == nil {
		return nil, errorNotFound()
	}
	if _, ok := m.entries[namespace]; !ok {
		return nil, errorNotFound()
	}
	if secret, ok := m.entries[namespace][name]; ok {
		return &secret, nil
	}
	return nil, errorNotFound()
}

func (m *mockSecretClient) List(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
	return nil, errorNotImplemented()
}

func (m *mockSecretClient) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, errorNotImplemented()
}

func (m *mockSecretClient) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Secret, err error) {
	return nil, errorNotImplemented()
}

// --------------------------

// mock node client interface

type mockNodeClient struct {
	nodes []v1.Node
}

func (m *mockNodeClient) Create(node *v1.Node) (*v1.Node, error) {
	return nil, errorNotImplemented()
}
func (m *mockNodeClient) Update(node *v1.Node) (*v1.Node, error) {
	return nil, errorNotImplemented()
}
func (m *mockNodeClient) UpdateStatus(node *v1.Node) (*v1.Node, error) {
	return nil, errorNotImplemented()
}
func (m *mockNodeClient) Delete(name string, options *metav1.DeleteOptions) error {
	return errorNotImplemented()
}
func (m *mockNodeClient) Get(name string, options metav1.GetOptions) (*v1.Node, error) {
	return nil, errorNotImplemented()
}
func (m *mockNodeClient) List(opts metav1.ListOptions) (*v1.NodeList, error) {
	return &v1.NodeList{Items: m.nodes}, nil
}
func (m *mockNodeClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return nil, errorNotImplemented()
}
func (m *mockNodeClient) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Node, err error) {
	return nil, errorNotImplemented()
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
	tempFile, err := ioutil.TempFile("", "node-password-test.*")
	if err != nil {
		log.Fatal(err)
	}
	tempFile.Close()

	var passwordEntries string
	for i := 1; i <= migrateNumNodes; i++ {
		passwordEntries += fmt.Sprintf("node%d,node%d\n", i, i)
	}
	if err := ioutil.WriteFile(tempFile.Name(), []byte(passwordEntries), 0600); err != nil {
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

func errorNotImplemented() error {
	log.Fatal("not implemented")
	return apierrors.NewMethodNotSupported(schema.GroupResource{}, "not-implemented")
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

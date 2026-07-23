package nodepassword

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
)

// fakeSecrets implements secretOps for ensure() unit tests.
type fakeSecrets struct {
	// secrets keyed by "namespace/name"
	items map[string]*v1.Secret
	// create behavior
	createErr   error
	createCount int
	// get for uncached path only; if nil, uses items
}

func (f *fakeSecrets) key(ns, name string) string { return ns + "/" + name }

func (f *fakeSecrets) Create(s *v1.Secret) (*v1.Secret, error) {
	f.createCount++
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.items == nil {
		f.items = map[string]*v1.Secret{}
	}
	k := f.key(s.Namespace, s.Name)
	if _, ok := f.items[k]; ok {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, s.Name)
	}
	cp := s.DeepCopy()
	f.items[k] = cp
	return cp, nil
}

func (f *fakeSecrets) Get(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
	if f.items == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	s, ok := f.items[f.key(namespace, name)]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
	}
	return s.DeepCopy(), nil
}

func (f *fakeSecrets) Update(s *v1.Secret) (*v1.Secret, error) {
	return s, nil
}

func (f *fakeSecrets) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	delete(f.items, f.key(namespace, name))
	return nil
}

func (f *fakeSecrets) List(namespace string, _ metav1.ListOptions) (*v1.SecretList, error) {
	return &v1.SecretList{}, nil
}

func secretWithHash(nodeName, pass string) *v1.Secret {
	h, err := Hasher.CreateHash(pass)
	if err != nil {
		panic(err)
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSecretName(nodeName),
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string][]byte{"hash": []byte(h)},
		Type: SecretTypeNodePassword,
	}
}

func newTestController(store toolscache.Store, secrets secretOps) *nodePasswordController {
	return &nodePasswordController{
		secrets:      secrets,
		secretsStore: store,
	}
}

func storeWithSecret(s *v1.Secret) toolscache.Store {
	store := toolscache.NewStore(toolscache.MetaNamespaceKeyFunc)
	if s != nil {
		if err := store.Add(s); err != nil {
			panic(err)
		}
	}
	return store
}

func Test_UnitEnsure_CacheHit(t *testing.T) {
	const node, pass = "node-a", "secret-pass"
	sec := secretWithHash(node, pass)
	secrets := &fakeSecrets{items: map[string]*v1.Secret{}}
	npc := newTestController(storeWithSecret(sec), secrets)

	if err := npc.ensure(node, pass); err != nil {
		t.Fatalf("ensure cache hit: %v", err)
	}
	if secrets.createCount != 0 {
		t.Fatalf("create called %d times on cache hit", secrets.createCount)
	}
}

func Test_UnitEnsure_CacheMissApiserverHit(t *testing.T) {
	const node, pass = "node-b", "secret-pass"
	sec := secretWithHash(node, pass)
	// Cache empty; apiserver has secret (stale cache after restart).
	secrets := &fakeSecrets{items: map[string]*v1.Secret{
		metav1.NamespaceSystem + "/" + getSecretName(node): sec,
	}}
	npc := newTestController(storeWithSecret(nil), secrets)

	if err := npc.ensure(node, pass); err != nil {
		t.Fatalf("ensure apiserver hit: %v", err)
	}
	if secrets.createCount != 0 {
		t.Fatalf("create called %d times when secret exists on apiserver", secrets.createCount)
	}
}

func Test_UnitEnsure_CreateWhenMissing(t *testing.T) {
	const node, pass = "node-c", "secret-pass"
	secrets := &fakeSecrets{items: map[string]*v1.Secret{}}
	npc := newTestController(storeWithSecret(nil), secrets)

	if err := npc.ensure(node, pass); err != nil {
		t.Fatalf("ensure create: %v", err)
	}
	if secrets.createCount != 1 {
		t.Fatalf("create count = %d, want 1", secrets.createCount)
	}
	// Second ensure should hit apiserver (and/or we can put in store).
	if err := npc.ensure(node, pass); err != nil {
		t.Fatalf("ensure after create: %v", err)
	}
}

func Test_UnitEnsure_AlreadyExistsRace(t *testing.T) {
	const node, pass = "node-d", "secret-pass"
	// Cache empty and apiserver empty until Create; Create returns AlreadyExists
	// and installs the peer-created secret for the follow-up uncached verify.
	racer := &raceSecrets{pass: pass, node: node}
	npc := newTestController(storeWithSecret(nil), racer)

	if err := npc.ensure(node, pass); err != nil {
		t.Fatalf("ensure AlreadyExists race: %v", err)
	}
	if racer.createCount != 1 {
		t.Fatalf("create count = %d, want 1", racer.createCount)
	}
	if racer.verifyAfterCreate < 1 {
		t.Fatalf("expected post-create verify against apiserver")
	}
}

// raceSecrets: apiserver empty until Create; Create returns AlreadyExists and
// installs secret so subsequent Get succeeds (lost create race).
type raceSecrets struct {
	node              string
	pass              string
	createCount       int
	verifyAfterCreate int
	installed         *v1.Secret
}

func (r *raceSecrets) Create(s *v1.Secret) (*v1.Secret, error) {
	r.createCount++
	// Peer created first
	r.installed = secretWithHash(r.node, r.pass)
	return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, s.Name)
}

func (r *raceSecrets) Get(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
	if r.installed != nil {
		r.verifyAfterCreate++
		return r.installed.DeepCopy(), nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

func (r *raceSecrets) Update(s *v1.Secret) (*v1.Secret, error) { return s, nil }
func (r *raceSecrets) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	return nil
}
func (r *raceSecrets) List(namespace string, _ metav1.ListOptions) (*v1.SecretList, error) {
	return &v1.SecretList{}, nil
}

func Test_UnitSecretNotFound(t *testing.T) {
	assertEqual(t, secretNotFound(errorNotFound()), true)
	assertEqual(t, secretNotFound(fmt.Errorf("wrap: %w", errorNotFound())), true)
	// passwordError wrapping NotFound (as returned by verifyHash)
	err := &passwordError{node: "n", err: errorNotFound()}
	assertEqual(t, secretNotFound(err), true)
	assertEqual(t, secretNotFound(errorAlreadyExists()), false)
	assertEqual(t, secretNotFound(nil), false)
}

func Test_UnitEnsure_WrongPassword(t *testing.T) {
	const node = "node-e"
	sec := secretWithHash(node, "correct")
	secrets := &fakeSecrets{items: map[string]*v1.Secret{}}
	npc := newTestController(storeWithSecret(sec), secrets)

	err := npc.ensure(node, "wrong")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if secrets.createCount != 0 {
		t.Fatalf("must not create when password verify fails: creates=%d", secrets.createCount)
	}
}

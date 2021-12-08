package file

import (
	"encoding/json"
	"os"

	"github.com/rancher/dynamiclistener"
	v1 "k8s.io/api/core/v1"
)

func New(file string) dynamiclistener.TLSStorage {
	return &storage{
		file: file,
	}
}

type storage struct {
	file string
}

func (s *storage) Get() (*v1.Secret, error) {
	f, err := os.Open(s.file)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	secret := v1.Secret{}
	return &secret, json.NewDecoder(f).Decode(&secret)
}

func (s *storage) Update(secret *v1.Secret) error {
	f, err := os.Create(s.file)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(secret)
}

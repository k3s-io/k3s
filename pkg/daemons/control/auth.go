package control

import (
	"github.com/rancher/k3s/pkg/authenticator/basicauth"
	"github.com/rancher/k3s/pkg/authenticator/passwordfile"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/union"
)

func basicAuthenticator(basicAuthFile string) (authenticator.Request, error) {
	if basicAuthFile == "" {
		return nil, nil
	}
	basicAuthenticator, err := passwordfile.NewCSV(basicAuthFile)
	if err != nil {
		return nil, err
	}
	return basicauth.New(basicAuthenticator), nil
}

func combineAuthenticators(auths ...authenticator.Request) authenticator.Request {
	var authenticators []authenticator.Request
	for _, auth := range auths {
		if auth != nil {
			authenticators = append(authenticators, auth)
		}
	}
	return group.NewAuthenticatedGroupAdder(union.New(authenticators...))
}

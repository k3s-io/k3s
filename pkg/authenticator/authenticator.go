package authenticator

import (
	"strings"

	"github.com/k3s-io/k3s/pkg/authenticator/basicauth"
	"github.com/k3s-io/k3s/pkg/authenticator/passwordfile"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

func FromArgs(args []string) (authenticator.Request, error) {
	var authenticators []authenticator.Request
	basicFile := getArg("--basic-auth-file", args)
	if basicFile != "" {
		basicAuthenticator, err := passwordfile.NewCSV(basicFile)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, basicauth.New(basicAuthenticator))
	}

	clientCA := getArg("--client-ca-file", args)
	if clientCA != "" {
		ca, err := dynamiccertificates.NewDynamicCAContentFromFile("client-ca", clientCA)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, x509.NewDynamic(ca.VerifyOptions, x509.CommonNameUserConversion))
	}

	return Combine(authenticators...), nil
}

func getArg(key string, args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, key) {
			continue
		}
		return strings.SplitN(arg, "=", 2)[1]
	}
	return ""
}

func Combine(auths ...authenticator.Request) authenticator.Request {
	var authenticators []authenticator.Request
	for _, auth := range auths {
		if auth != nil {
			authenticators = append(authenticators, auth)
		}
	}
	return group.NewAuthenticatedGroupAdder(union.New(authenticators...))
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/passwd"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
)

type ServerTokenRequest struct {
	Action *string `json:"stage,omitempty"`
}

func getServerTokenRequest(req *http.Request) (ServerTokenRequest, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return ServerTokenRequest{}, err
	}
	result := ServerTokenRequest{}
	err = json.Unmarshal(b, &result)
	return result, err
}

func tokenRequestHandler(ctx context.Context, server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil || req.Method != http.MethodPut {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		logrus.Info("Received token request")
		var err error
		b := []byte{}
		var token string
		sTokenReq, err := getServerTokenRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}

		if *sTokenReq.Action == "rotate" {
			token, err = tokenRotate(ctx, server)
			b = []byte(token)
		} else {
			err = fmt.Errorf("unknown action %s requested", *sTokenReq.Action)
		}

		if err != nil {
			genErrorMessage(resp, http.StatusInternalServerError, err, "token")
			return
		}
		resp.WriteHeader(http.StatusOK)
		resp.Write(b)
	})
}

func tokenRotate(ctx context.Context, server *config.Control) (string, error) {
	passwd, err := passwd.Read(server.Runtime.PasswdFile)
	if err != nil {
		return "", err
	}

	logrus.Info("BEFORE ", passwd)
	if err != nil {
		return "", err
	}
	oldToken, found := passwd.Pass("server")
	if !found {
		return "", fmt.Errorf("server token not found")
	}
	newToken, err := util.Random(16)
	if err != nil {
		return "", err
	}
	// if err := passwd.EnsureUser("server", version.Program+":server", newToken); err != nil {
	// 	return "", err
	// }
	// logrus.Info("AFTER ", passwd)

	// if err := passwd.Write(server.Runtime.PasswdFile); err != nil {
	// 	return "", err
	// }

	serverTokenFile := filepath.Join(server.DataDir, "token")
	b, err := os.ReadFile(serverTokenFile)
	if err != nil {
		return "", err
	}
	logrus.Info("OLD TOKEN ", string(b))
	if err := writeToken("server:"+newToken, serverTokenFile, server.Runtime.ServerCA); err != nil {
		return "", err
	}

	b, err = os.ReadFile(serverTokenFile)
	if err != nil {
		return "", err
	}
	logrus.Info("NEW TOKEN ", string(b))

	cluster.RotateBootstrapToken(ctx, server, oldToken)
	// if err := cluster.Save(ctx, server, true); err != nil {
	// 	return "", err
	// }
	return newToken, nil
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/passwd"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
)

type ServerTokenRequest struct {
	Action   *string `json:"stage,omitempty"`
	NewToken *string `json:"newToken,omitempty"`
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
		var err error
		sTokenReq, err := getServerTokenRequest(req)
		logrus.Info("Received token request: ", *sTokenReq.Action, *sTokenReq.NewToken)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}
		if *sTokenReq.Action == "rotate" {
			err = tokenRotate(ctx, server, *sTokenReq.NewToken)
		} else {
			err = fmt.Errorf("unknown action %s requested", *sTokenReq.Action)
		}

		if err != nil {
			genErrorMessage(resp, http.StatusInternalServerError, err, "token")
			return
		}
		resp.WriteHeader(http.StatusOK)
	})
}

func tokenRotate(ctx context.Context, server *config.Control, newToken string) error {
	passwd, err := passwd.Read(server.Runtime.PasswdFile)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	oldToken, found := passwd.Pass("server")
	if !found {
		return fmt.Errorf("server token not found")
	}
	if newToken == "" {
		newToken, err = util.Random(16)
		if err != nil {
			return err
		}
	}

	// if err := passwd.EnsureUser("server", version.Program+":server", newToken); err != nil {
	// 	return err
	// }

	if err := passwd.Write(server.Runtime.PasswdFile); err != nil {
		return err
	}

	serverTokenFile := filepath.Join(server.DataDir, "token")
	if err := writeToken("server:"+newToken, serverTokenFile, server.Runtime.ServerCA); err != nil {
		return err
	}

	if err := cluster.RotateBootstrapToken(ctx, server, oldToken); err != nil {
		return err
	}
	server.Token = newToken
	return cluster.Save(ctx, server, true)
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/passwd"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

type TokenRotateRequest struct {
	NewToken *string `json:"newToken,omitempty"`
}

func getServerTokenRequest(req *http.Request) (TokenRotateRequest, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return TokenRotateRequest{}, err
	}
	result := TokenRotateRequest{}
	err = json.Unmarshal(b, &result)
	return result, err
}

func TokenRequest(ctx context.Context, control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			util.SendError(fmt.Errorf("method not allowed"), resp, req, http.StatusMethodNotAllowed)
			return
		}
		var err error
		sTokenReq, err := getServerTokenRequest(req)
		logrus.Debug("Received token request")
		if err != nil {
			util.SendError(err, resp, req, http.StatusBadRequest)
			return
		}
		if err = tokenRotate(ctx, control, *sTokenReq.NewToken); err != nil {
			util.SendErrorWithID(err, "token", resp, req, http.StatusInternalServerError)
			return
		}
		resp.WriteHeader(http.StatusOK)
	})
}

func WriteToken(token, file, certs string) error {
	if len(token) == 0 {
		return nil
	}

	token, err := clientaccess.FormatToken(token, certs)
	if err != nil {
		return err
	}
	return os.WriteFile(file, []byte(token+"\n"), 0600)
}

func tokenRotate(ctx context.Context, control *config.Control, newToken string) error {
	passwd, err := passwd.Read(control.Runtime.PasswdFile)
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

	if err := passwd.EnsureUser("server", version.Program+":server", newToken); err != nil {
		return err
	}

	// If the agent token is the same a server, we need to change both
	if agentToken, found := passwd.Pass("node"); found && agentToken == oldToken && control.AgentToken == "" {
		if err := passwd.EnsureUser("node", version.Program+":agent", newToken); err != nil {
			return err
		}
	}

	if err := passwd.Write(control.Runtime.PasswdFile); err != nil {
		return err
	}

	serverTokenFile := filepath.Join(control.DataDir, "token")
	if err := WriteToken("server:"+newToken, serverTokenFile, control.Runtime.ServerCA); err != nil {
		return err
	}

	if err := cluster.RotateBootstrapToken(ctx, control, oldToken); err != nil {
		return err
	}
	control.Token = newToken
	return cluster.Save(ctx, control, true)
}

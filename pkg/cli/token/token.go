package token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/kubeadm"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/server/handlers"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/duration"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/utils/ptr"
)

func Create(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return create(app, &cmds.TokenConfig)
}

func create(app *cli.Context, cfg *cmds.Token) error {
	if err := kubeadm.SetDefaults(app, cfg); err != nil {
		return err
	}

	cfg.Kubeconfig = util.GetKubeConfigPath(cfg.Kubeconfig)
	client, err := util.GetClientSet(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	restConfig, err := util.GetRESTConfig(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	if len(restConfig.TLSClientConfig.CAData) == 0 && restConfig.TLSClientConfig.CAFile != "" {
		restConfig.TLSClientConfig.CAData, err = os.ReadFile(restConfig.TLSClientConfig.CAFile)
		if err != nil {
			return err
		}
	}

	bts, err := kubeadm.NewBootstrapTokenString(cfg.Token)
	if err != nil {
		return err
	}

	bt := kubeadm.BootstrapToken{
		Token:       bts,
		Description: cfg.Description,
		TTL:         &metav1.Duration{Duration: cfg.TTL},
		Usages:      cfg.Usages,
		Groups:      cfg.Groups,
	}

	secretName := bootstraputil.BootstrapTokenSecretName(bt.Token.ID)
	if secret, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Get(context.TODO(), secretName, metav1.GetOptions{}); secret != nil && err == nil {
		return fmt.Errorf("a token with id %q already exists", bt.Token.ID)
	}

	secret := kubeadm.BootstrapTokenToSecret(&bt)
	if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return err
	}

	token, err := clientaccess.FormatTokenBytes(bt.Token.String(), restConfig.TLSClientConfig.CAData)
	if err != nil {
		return err
	}

	fmt.Println(token)
	return nil
}

func Delete(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return delete(app, &cmds.TokenConfig)
}

func delete(app *cli.Context, cfg *cmds.Token) error {
	args := app.Args()
	if len(args) < 1 {
		return errors.New("missing argument; 'token delete' is missing token")
	}

	cfg.Kubeconfig = util.GetKubeConfigPath(cfg.Kubeconfig)
	client, err := util.GetClientSet(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	for _, token := range args {
		if !bootstraputil.IsValidBootstrapTokenID(token) {
			bts, err := kubeadm.NewBootstrapTokenString(cfg.Token)
			if err != nil {
				return fmt.Errorf("given token didn't match pattern %q or %q", bootstrapapi.BootstrapTokenIDPattern, bootstrapapi.BootstrapTokenIDPattern)
			}
			token = bts.ID
		}
		secretName := bootstraputil.BootstrapTokenSecretName(token)
		if err := client.CoreV1().Secrets(metav1.NamespaceSystem).Delete(context.TODO(), secretName, metav1.DeleteOptions{}); err != nil {
			return errors.Wrapf(err, "failed to delete bootstrap token %q", err)
		}

		fmt.Printf("bootstrap token %q deleted\n", token)
	}
	return nil
}

func Generate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return generate(app, &cmds.TokenConfig)
}

func generate(app *cli.Context, cfg *cmds.Token) error {
	token, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	fmt.Println("\033[33mWARNING\033[0m: Recommended to keep a record of the old token. If restoring from a snapshot, you must use the token associated with that snapshot.")
	info, err := serverAccess(&cmds.TokenConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(handlers.TokenRotateRequest{
		NewToken: ptr.To(cmds.TokenConfig.NewToken),
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/token", b); err != nil {
		return err
	}
	// wait for etcd db propagation delay
	time.Sleep(1 * time.Second)
	fmt.Println("Token rotated, restart", version.Program, "nodes with new token")
	return nil
}

func serverAccess(cfg *cmds.Token) (*clientaccess.Info, error) {
	// hide process arguments from ps output, since they likely contain tokens.
	proctitle.SetProcTitle(os.Args[0] + " token")

	dataDir, err := server.ResolveDataDir("")
	if err != nil {
		return nil, err
	}

	if cfg.Token == "" {
		fp := filepath.Join(dataDir, "token")
		tokenByte, err := os.ReadFile(fp)
		if err != nil {
			return nil, err
		}
		cfg.Token = string(bytes.TrimRight(tokenByte, "\n"))
	}
	return clientaccess.ParseAndValidateToken(cfg.ServerURL, cfg.Token, clientaccess.WithUser("server"))
}

func List(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return list(app, &cmds.TokenConfig)
}

func list(app *cli.Context, cfg *cmds.Token) error {
	if err := kubeadm.SetDefaults(app, cfg); err != nil {
		return err
	}

	cfg.Kubeconfig = util.GetKubeConfigPath(cfg.Kubeconfig)
	client, err := util.GetClientSet(cfg.Kubeconfig)
	if err != nil {
		return err
	}

	tokenSelector := fields.SelectorFromSet(
		map[string]string{
			"type": string(bootstrapapi.SecretTypeBootstrapToken),
		},
	)
	listOptions := metav1.ListOptions{
		FieldSelector: tokenSelector.String(),
	}

	secrets, err := client.CoreV1().Secrets(metav1.NamespaceSystem).List(context.TODO(), listOptions)
	if err != nil {
		return errors.Wrapf(err, "failed to list bootstrap tokens")
	}

	tokens := make([]*kubeadm.BootstrapToken, len(secrets.Items))
	for i, secret := range secrets.Items {
		token, err := kubeadm.BootstrapTokenFromSecret(&secret)
		if err != nil {
			fmt.Printf("%v", err)
			continue
		}
		tokens[i] = token
	}

	switch cfg.Output {
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(tokens); err != nil {
			return err
		}
		return nil
	case "yaml":
		if err := yaml.NewEncoder(os.Stdout).Encode(tokens); err != nil {
			return err
		}
		return nil
	default:
		format := "%s\t%s\t%s\t%s\t%s\t%s\n"
		w := tabwriter.NewWriter(os.Stdout, 10, 4, 3, ' ', 0)
		defer w.Flush()

		fmt.Fprintf(w, format, "TOKEN", "TTL", "EXPIRES", "USAGES", "DESCRIPTION", "EXTRA GROUPS")
		for _, token := range tokens {
			ttl := "<forever>"
			expires := "<never>"
			if token.Expires != nil {
				ttl = duration.ShortHumanDuration(token.Expires.Sub(time.Now()))
				expires = token.Expires.Format(time.RFC3339)
			}

			fmt.Fprintf(w, format, token.Token.ID, ttl, expires, joinOrNone(token.Usages...), joinOrNone(token.Description), joinOrNone(token.Groups...))
		}
	}

	return nil
}

// joinOrNone joins strings with a comma. If the resulting output is an empty string,
// it instead returns the replacement string "<none>"
func joinOrNone(s ...string) string {
	j := strings.Join(s, ",")
	if j == "" {
		return "<none>"
	}
	return j
}

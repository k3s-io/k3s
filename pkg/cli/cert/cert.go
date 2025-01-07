package cert

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/util/services"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) (string, error) {
	proctitle.SetProcTitle(os.Args[0])

	dataDir, err := datadir.Resolve(cfg.DataDir)
	if err != nil {
		return "", err
	}
	sc.ControlConfig.DataDir = filepath.Join(dataDir, "server")

	if cfg.Token == "" {
		fp := filepath.Join(sc.ControlConfig.DataDir, "token")
		tokenByte, err := os.ReadFile(fp)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		cfg.Token = string(bytes.TrimRight(tokenByte, "\n"))
	}
	sc.ControlConfig.Token = cfg.Token
	sc.ControlConfig.Runtime = config.NewRuntime(nil)

	return dataDir, nil
}

func Check(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return check(app, &cmds.ServerConfig)
}

func check(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	_, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	deps.CreateRuntimeCertFiles(&serverConfig.ControlConfig)

	if err := validateCertConfig(); err != nil {
		return err
	}

	if len(cmds.ServicesList) == 0 {
		// detecting if the command is being run on an agent or server based on presence of the server data-dir
		_, err := os.Stat(serverConfig.ControlConfig.DataDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			logrus.Infof("Agent detected, checking agent certificates")
			cmds.ServicesList = services.Agent
		} else {
			logrus.Infof("Server detected, checking agent and server certificates")
			cmds.ServicesList = services.All
		}
	}

	fileMap, err := services.FilesForServices(serverConfig.ControlConfig, cmds.ServicesList)
	if err != nil {
		return err
	}

	now := time.Now()
	warn := now.Add(time.Hour * 24 * config.CertificateRenewDays)
	outFmt := app.String("output")
	switch outFmt {
	case "text":
		for service, files := range fileMap {
			logrus.Info("Checking certificates for " + service)
			for _, file := range files {
				// ignore errors, as some files may not exist, or may not contain certs.
				// Only check whatever exists and has certs.
				certs, _ := certutil.CertsFromFile(file)
				for _, cert := range certs {
					if now.Before(cert.NotBefore) {
						logrus.Errorf("%s: certificate %s is not valid before %s", file, cert.Subject, cert.NotBefore.Format(time.RFC3339))
					} else if now.After(cert.NotAfter) {
						logrus.Errorf("%s: certificate %s expired at %s", file, cert.Subject, cert.NotAfter.Format(time.RFC3339))
					} else if warn.After(cert.NotAfter) {
						logrus.Warnf("%s: certificate %s will expire within %d days at %s", file, cert.Subject, config.CertificateRenewDays, cert.NotAfter.Format(time.RFC3339))
					} else {
						logrus.Infof("%s: certificate %s is ok, expires at %s", file, cert.Subject, cert.NotAfter.Format(time.RFC3339))
					}
				}
			}
		}
	case "table":
		var tabBuffer bytes.Buffer
		w := tabwriter.NewWriter(&tabBuffer, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "CERTIFICATE\tSUBJECT\tSTATUS\tEXPIRES\n")
		fmt.Fprintf(w, "-----------\t-------\t------\t-------")
		for _, files := range fileMap {
			for _, file := range files {
				certs, _ := certutil.CertsFromFile(file)
				for _, cert := range certs {
					baseName := filepath.Base(file)
					var status string
					expiration := cert.NotAfter.Format(time.RFC3339)
					if now.Before(cert.NotBefore) {
						status = "NOT YET VALID"
						expiration = cert.NotBefore.Format(time.RFC3339)
					} else if now.After(cert.NotAfter) {
						status = "EXPIRED"
					} else if warn.After(cert.NotAfter) {
						status = "WARNING"
					} else {
						status = "OK"
					}
					fmt.Fprintf(w, "\n%s\t%s\t%s\t%s", baseName, cert.Subject, status, expiration)
				}
			}
		}
		w.Flush()
		fmt.Println(tabBuffer.String())
	default:
		return fmt.Errorf("invalid output format %s", outFmt)
	}
	return nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return rotate(app, &cmds.ServerConfig)
}

func rotate(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	deps.CreateRuntimeCertFiles(&serverConfig.ControlConfig)

	if err := validateCertConfig(); err != nil {
		return err
	}

	if len(cmds.ServicesList) == 0 {
		// detecting if the command is being run on an agent or server based on presence of the server data-dir
		_, err := os.Stat(serverConfig.ControlConfig.DataDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			logrus.Infof("Agent detected, rotating agent certificates")
			cmds.ServicesList = services.Agent
		} else {
			logrus.Infof("Server detected, rotating agent and server certificates")
			cmds.ServicesList = services.All
		}
	}

	fileMap, err := services.FilesForServices(serverConfig.ControlConfig, cmds.ServicesList)
	if err != nil {
		return err
	}

	// back up all the files
	agentDataDir := filepath.Join(dataDir, "agent")
	tlsBackupDir, err := backupCertificates(serverConfig.ControlConfig.DataDir, agentDataDir, fileMap)
	if err != nil {
		return err
	}

	// The dynamiclistener cache file can't be simply deleted, we need to create a trigger
	// file to indicate that the cert needs to be regenerated on startup.
	for _, service := range cmds.ServicesList {
		if service == version.Program+services.ProgramServer {
			dynamicListenerRegenFilePath := filepath.Join(serverConfig.ControlConfig.DataDir, "tls", "dynamic-cert-regenerate")
			if err := os.WriteFile(dynamicListenerRegenFilePath, []byte{}, 0600); err != nil {
				return err
			}
			logrus.Infof("Rotating dynamic listener certificate")
		}
	}

	// remove all files
	for service, files := range fileMap {
		logrus.Info("Rotating certificates for " + service)
		for _, file := range files {
			if err := os.Remove(file); err == nil {
				logrus.Debugf("file %s is deleted", file)
			}
		}
	}
	logrus.Infof("Successfully backed up certificates to %s, please restart %s server or agent to rotate certificates", tlsBackupDir, version.Program)
	return nil
}

func backupCertificates(serverDataDir, agentDataDir string, fileMap map[string][]string) (string, error) {
	backupDirName := fmt.Sprintf("tls-%d", time.Now().Unix())
	serverTLSDir := filepath.Join(serverDataDir, "tls")
	tlsBackupDir := filepath.Join(agentDataDir, backupDirName)

	// backup the server TLS dir if it exists
	if _, err := os.Stat(serverTLSDir); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
	} else {
		tlsBackupDir = filepath.Join(serverDataDir, backupDirName)
		if err := copy.Copy(serverTLSDir, tlsBackupDir); err != nil {
			return "", err
		}
	}

	for _, files := range fileMap {
		for _, file := range files {
			if strings.HasPrefix(file, agentDataDir) {
				cert := filepath.Base(file)
				tlsBackupCert := filepath.Join(tlsBackupDir, cert)
				if err := util.CopyFile(file, tlsBackupCert, true); err != nil {
					return "", err
				}
			}
		}
	}

	return tlsBackupDir, nil
}

func validateCertConfig() error {
	for _, s := range cmds.ServicesList {
		if !services.IsValid(s) {
			return errors.New("service " + s + " is not recognized")
		}
	}
	return nil
}

func RotateCA(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return rotateCA(app, &cmds.ServerConfig, &cmds.CertRotateCAConfig)
}

func rotateCA(app *cli.Context, cfg *cmds.Server, sync *cmds.CertRotateCA) error {
	var serverConfig server.Config

	_, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	info, err := clientaccess.ParseAndValidateToken(cmds.ServerConfig.ServerURL, serverConfig.ControlConfig.Token, clientaccess.WithUser("server"))
	if err != nil {
		return err
	}

	// Set up dummy server config for reading new bootstrap data from disk.
	tmpServer := &config.Control{
		Runtime: config.NewRuntime(nil),
		DataDir: sync.CACertPath,
	}
	deps.CreateRuntimeCertFiles(tmpServer)

	// Override these paths so that we don't get warnings when they don't exist, as the user is not expected to provide them.
	tmpServer.Runtime.PasswdFile = "/dev/null"
	tmpServer.Runtime.IPSECKey = "/dev/null"

	buf := &bytes.Buffer{}
	if err := bootstrap.ReadFromDisk(buf, &tmpServer.Runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}

	url := fmt.Sprintf("/v1-%s/cert/cacerts?force=%t", version.Program, sync.Force)
	if err = info.Put(url, buf.Bytes()); err != nil {
		return errors.Wrap(err, "see server log for details")
	}

	fmt.Println("certificates saved to datastore")
	return nil
}

package cert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/server"
	k3sutil "github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/services"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
	pkgerrors "github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// Certificate defines a single certificate data structure
type Certificate struct {
	Filename     string
	Subject      string
	Issuer       string
	Usages       []string
	ExpiryTime   time.Time
	ResidualTime time.Duration
	Status       string // "OK", "WARNING", "EXPIRED", "NOT YET VALID"
}

// CertificateInfo defines the structure for storing certificate information
type CertificateInfo struct {
	Certificates  []Certificate
	ReferenceTime time.Time `json:"-" yaml:"-"`
}

// collectCertInfo collects information about certificates
func collectCertInfo(controlConfig config.Control, servicesList []string) (*CertificateInfo, error) {
	result := &CertificateInfo{}
	now := time.Now()
	warn := now.Add(time.Hour * 24 * config.CertificateRenewDays)

	fileMap, err := services.FilesForServices(controlConfig, servicesList)
	if err != nil {
		return nil, err
	}

	for _, files := range fileMap {
		for _, file := range files {
			certs, err := certutil.CertsFromFile(file)
			if err != nil {
				logrus.Debugf("%v", err)
				continue
			}

			for _, cert := range certs {
				expiration := cert.NotAfter
				status := k3sutil.GetCertStatus(cert, now, warn)
				if status == k3sutil.CertStatusNotYetValid {
					expiration = cert.NotBefore
				}
				usages := k3sutil.GetCertUsages(cert)
				result.Certificates = append(result.Certificates, Certificate{
					Filename:     filepath.Base(file),
					Subject:      cert.Subject.CommonName,
					Issuer:       cert.Issuer.CommonName,
					Usages:       usages,
					ExpiryTime:   expiration,
					ResidualTime: cert.NotAfter.Sub(now),
					Status:       status,
				})
			}
		}
	}
	result.ReferenceTime = now
	return result, nil
}

// Formatter defines the interface for formatting certificate information
type Formatter interface {
	Format(*CertificateInfo) error
}

// TextFormatter implements text format output
type TextFormatter struct {
	Writer io.Writer
}

func (f *TextFormatter) Format(certInfo *CertificateInfo) error {
	for _, cert := range certInfo.Certificates {
		usagesStr := strings.Join(cert.Usages, ",")
		switch cert.Status {
		case k3sutil.CertStatusNotYetValid:
			logrus.Errorf("%s: certificate %s (%s) is not valid before %s",
				cert.Filename, cert.Subject, usagesStr, cert.ExpiryTime.Format(time.RFC3339))
		case k3sutil.CertStatusExpired:
			logrus.Errorf("%s: certificate %s (%s) expired at %s",
				cert.Filename, cert.Subject, usagesStr, cert.ExpiryTime.Format(time.RFC3339))
		case k3sutil.CertStatusWarning:
			logrus.Warnf("%s: certificate %s (%s) will expire within %d days at %s",
				cert.Filename, cert.Subject, usagesStr, config.CertificateRenewDays, cert.ExpiryTime.Format(time.RFC3339))
		default:
			logrus.Infof("%s: certificate %s (%s) is ok, expires at %s",
				cert.Filename, cert.Subject, usagesStr, cert.ExpiryTime.Format(time.RFC3339))
		}
	}
	return nil
}

// TableFormatter implements table format output
type TableFormatter struct {
	Writer io.Writer
}

func (f *TableFormatter) Format(certInfo *CertificateInfo) error {
	w := tabwriter.NewWriter(f.Writer, 0, 0, 3, ' ', 0)
	now := certInfo.ReferenceTime
	defer w.Flush()

	fmt.Fprint(w, "\nFILENAME\tSUBJECT\tUSAGES\tEXPIRES\tRESIDUAL TIME\tSTATUS\n")
	fmt.Fprint(w, "--------\t-------\t------\t-------\t-------------\t------\n")

	for _, cert := range certInfo.Certificates {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			cert.Filename,
			cert.Subject,
			strings.Join(cert.Usages, ","),
			cert.ExpiryTime.Format("Jan 02, 2006 15:04 MST"),
			humanize.RelTime(now, now.Add(cert.ResidualTime), "", ""),
			cert.Status)
	}
	return nil
}

// JSONFormatter implements JSON format output
type JSONFormatter struct {
	Writer io.Writer
}

func (f *JSONFormatter) Format(certInfo *CertificateInfo) error {
	return json.NewEncoder(f.Writer).Encode(certInfo)
}

// YAMLFormatter implements YAML format output
type YAMLFormatter struct {
	Writer io.Writer
}

func (f *YAMLFormatter) Format(certInfo *CertificateInfo) error {
	return yaml.NewEncoder(f.Writer).Encode(certInfo)
}

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
	sc.ControlConfig.Runtime = config.NewRuntime()

	return dataDir, nil
}

func Check(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return check(app, &cmds.ServerConfig)
}

// check checks the status of the certificates
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

	if len(cmds.ServicesList.Value()) == 0 {
		// detecting if the command is being run on an agent or server based on presence of the server data-dir
		_, err := os.Stat(serverConfig.ControlConfig.DataDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			logrus.Infof("Agent detected, checking agent certificates")
			cmds.ServicesList = *cli.NewStringSlice(services.Agent...)
		} else {
			logrus.Infof("Server detected, checking agent and server certificates")
			cmds.ServicesList = *cli.NewStringSlice(services.All...)
		}
	}

	certInfo, err := collectCertInfo(serverConfig.ControlConfig, cmds.ServicesList.Value())
	if err != nil {
		return err
	}
	outFmt := app.String("output")

	var formatter Formatter
	switch outFmt {
	case "text":
		formatter = &TextFormatter{Writer: os.Stdout}
	case "table":
		formatter = &TableFormatter{Writer: os.Stdout}
	case "json":
		formatter = &JSONFormatter{Writer: os.Stdout}
	case "yaml":
		formatter = &YAMLFormatter{Writer: os.Stdout}
	default:
		return fmt.Errorf("invalid output format %s", outFmt)
	}

	return formatter.Format(certInfo)
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

	if len(cmds.ServicesList.Value()) == 0 {
		// detecting if the command is being run on an agent or server based on presence of the server data-dir
		_, err := os.Stat(serverConfig.ControlConfig.DataDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			logrus.Infof("Agent detected, rotating agent certificates")
			cmds.ServicesList = *cli.NewStringSlice(services.Agent...)
		} else {
			logrus.Infof("Server detected, rotating agent and server certificates")
			cmds.ServicesList = *cli.NewStringSlice(services.All...)
		}
	}

	fileMap, err := services.FilesForServices(serverConfig.ControlConfig, cmds.ServicesList.Value())
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
	for _, service := range cmds.ServicesList.Value() {
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
	for _, s := range cmds.ServicesList.Value() {
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
		Runtime: config.NewRuntime(),
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
		return pkgerrors.WithMessage(err, "see server log for details")
	}

	fmt.Println("certificates saved to datastore")
	return nil
}

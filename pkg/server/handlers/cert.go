package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/keyutil"
)

func CACertReplace(control *config.Control) http.HandlerFunc {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			util.SendError(fmt.Errorf("method not allowed"), resp, req, http.StatusMethodNotAllowed)
			return
		}
		force, _ := strconv.ParseBool(req.FormValue("force"))
		if err := caCertReplace(control, req.Body, force); err != nil {
			util.SendErrorWithID(err, "certificate", resp, req, http.StatusInternalServerError)
			return
		}
		logrus.Infof("certificate: Cluster Certificate Authority data has been updated, %s must be restarted.", version.Program)
		resp.WriteHeader(http.StatusNoContent)
	})
}

// caCertReplace stores new CA Certificate data from the client.  The data is temporarily written out to disk,
// validated to confirm that the new certs share a common root with the existing certs, and if so are saved to
// the datastore.  If the functions succeeds, servers should be restarted immediately to load the new certs
// from the bootstrap data.
func caCertReplace(control *config.Control, buf io.ReadCloser, force bool) error {
	tmpdir, err := os.MkdirTemp(control.DataDir, ".rotate-ca-tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	runtime := config.NewRuntime()
	runtime.EtcdConfig = control.Runtime.EtcdConfig
	runtime.ServerToken = control.Runtime.ServerToken

	tmpControl := &config.Control{
		Runtime: runtime,
		Token:   control.Token,
		DataDir: tmpdir,
	}
	deps.CreateRuntimeCertFiles(tmpControl)

	bootstrapData := bootstrap.PathsDataformat{}
	if err := json.NewDecoder(buf).Decode(&bootstrapData); err != nil {
		return err
	}

	if err := bootstrap.WriteToDiskFromStorage(bootstrapData, &tmpControl.Runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}

	if err := defaultBootstrap(control, tmpControl); err != nil {
		return pkgerrors.WithMessage(err, "failed to set default bootstrap values")
	}

	if err := validateBootstrap(control, tmpControl); err != nil {
		if !force {
			return pkgerrors.WithMessage(err, "failed to validate new CA certificates and keys")
		}
		logrus.Warnf("Save of CA certificates and keys forced, ignoring validation errors: %v", err)
	}

	if err := cluster.Save(context.TODO(), tmpControl, true); err != nil {
		return err
	}

	dynamicListenerRegenFilePath := filepath.Join(control.DataDir, "tls", "dynamic-cert-regenerate")
	if err := os.WriteFile(dynamicListenerRegenFilePath, []byte{}, 0600); err != nil {
		logrus.Warnf("Failed to create dynamic-cert-regenerate file: %v", err)
	}

	return nil
}

// defaultBootstrap provides default values from the existing bootstrap fields
// if the value is not tagged for rotation, or the current value is empty.
func defaultBootstrap(oldControl, newControl *config.Control) error {
	errs := []error{}
	// Use reflection to iterate over all of the bootstrap fields, checking files at each of the new paths.
	oldMeta := reflect.ValueOf(&oldControl.Runtime.ControlRuntimeBootstrap).Elem()
	newMeta := reflect.ValueOf(&newControl.Runtime.ControlRuntimeBootstrap).Elem()

	// use the existing file if the new file does not exist or is empty
	for _, field := range reflect.VisibleFields(oldMeta.Type()) {
		newVal := newMeta.FieldByName(field.Name)
		info, err := os.Stat(newVal.String())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, pkgerrors.WithMessage(err, field.Name))
			continue
		}

		if field.Tag.Get("rotate") != "true" || info == nil || info.Size() == 0 {
			if newVal.CanSet() {
				oldVal := oldMeta.FieldByName(field.Name)
				logrus.Infof("Using current data for %s: %s", field.Name, oldVal)
				newVal.Set(oldVal)
			} else {
				errs = append(errs, fmt.Errorf("cannot use current data for %s; field is not settable", field.Name))
			}
		}
	}
	return merr.NewErrors(errs...)
}

// validateBootstrap checks the new certs and keys to ensure that the cluster would function properly were they to be used.
// - The new leaf CA certificates must be verifiable using the same root and intermediate certs as the current leaf CA certificates.
// - The new service account signing key bundle must include the currently active signing key.
func validateBootstrap(oldControl, newControl *config.Control) error {
	errs := []error{}

	// Use reflection to iterate over all of the bootstrap fields, checking files at each of the new paths.
	oldMeta := reflect.ValueOf(&oldControl.Runtime.ControlRuntimeBootstrap).Elem()
	newMeta := reflect.ValueOf(&newControl.Runtime.ControlRuntimeBootstrap).Elem()

	for _, field := range reflect.VisibleFields(oldMeta.Type()) {
		// Only handle bootstrap fields tagged for rotation
		if field.Tag.Get("rotate") != "true" {
			continue
		}
		oldVal := oldMeta.FieldByName(field.Name)
		newVal := newMeta.FieldByName(field.Name)

		// Check CA chain consistency and cert/key agreement
		if strings.HasSuffix(field.Name, "CA") {
			if err := validateCA(oldVal.String(), newVal.String()); err != nil {
				errs = append(errs, pkgerrors.WithMessage(err, field.Name))
			}
			newKeyVal := newMeta.FieldByName(field.Name + "Key")
			oldKeyVal := oldMeta.FieldByName(field.Name + "Key")
			if err := validateCAKey(oldVal.String(), oldKeyVal.String(), newVal.String(), newKeyVal.String()); err != nil {
				errs = append(errs, pkgerrors.WithMessage(err, field.Name+"Key"))
			}
		}

		// Check signing key rotation
		if field.Name == "ServiceKey" {
			if err := validateServiceKey(oldVal.String(), newVal.String()); err != nil {
				errs = append(errs, pkgerrors.WithMessage(err, field.Name))
			}
		}
	}

	return merr.NewErrors(errs...)
}

func validateCA(oldCAPath, newCAPath string) error {
	// Skip validation if old values are being reused
	if oldCAPath == newCAPath {
		return nil
	}

	oldCerts, err := certutil.CertsFromFile(oldCAPath)
	if err != nil {
		return err
	}

	newCerts, err := certutil.CertsFromFile(newCAPath)
	if err != nil {
		return err
	}

	if len(newCerts) == 1 {
		return errors.New("new CA bundle contains only a single certificate but should include root or intermediate CA certificates")
	}

	roots := x509.NewCertPool()
	intermediates := x509.NewCertPool()

	// Load all certs from the old bundle
	for _, cert := range oldCerts {
		if len(cert.AuthorityKeyId) == 0 || bytes.Equal(cert.AuthorityKeyId, cert.SubjectKeyId) {
			roots.AddCert(cert)
		} else {
			intermediates.AddCert(cert)
		}
	}

	// Include any intermediates from the new bundle, in case they're cross-signed by a cert in the old bundle
	for i, cert := range newCerts {
		if i > 0 {
			if len(cert.AuthorityKeyId) > 0 {
				intermediates.AddCert(cert)
			}
		}
	}

	// Verify the first cert in the bundle, using the combined roots and intermediates
	_, err = newCerts[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates})
	if err != nil {
		err = pkgerrors.WithMessage(err, "new CA cert cannot be verified using old CA chain")
	}
	return err
}

// validateCAKey confirms that the private key is valid for the certificate
func validateCAKey(oldCAPath, oldCAKeyPath, newCAPath, newCAKeyPath string) error {
	// Skip validation if old values are being reused
	if oldCAPath == newCAPath && oldCAKeyPath == newCAKeyPath {
		return nil
	}

	_, err := tls.LoadX509KeyPair(newCAPath, newCAKeyPath)
	if err != nil {
		err = pkgerrors.WithMessage(err, "new CA cert and key cannot be loaded as X590KeyPair")
	}
	return err
}

// validateServiceKey ensures that the first key from the old serviceaccount signing key list
// is also present in the new key list, to ensure that old signatures can still be validated.
func validateServiceKey(oldKeyPath, newKeyPath string) error {
	oldKeys, err := keyutil.PublicKeysFromFile(oldKeyPath)
	if err != nil {
		return err
	}

	newKeys, err := keyutil.PublicKeysFromFile(newKeyPath)
	if err != nil {
		return err
	}

	for _, key := range newKeys {
		if reflect.DeepEqual(oldKeys[0], key) {
			return nil
		}
	}

	return errors.New("old ServiceAccount signing key not in new ServiceAccount key list")
}

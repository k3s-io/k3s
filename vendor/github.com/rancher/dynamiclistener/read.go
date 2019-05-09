package dynamiclistener

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

func ReadTLSConfig(userConfig *UserConfig) error {
	var err error

	path := userConfig.CertPath

	userConfig.CACerts, err = readPEM(filepath.Join(path, "cacerts.pem"))
	if err != nil {
		return err
	}

	userConfig.Key, err = readPEM(filepath.Join(path, "key.pem"))
	if err != nil {
		return err
	}

	userConfig.Cert, err = readPEM(filepath.Join(path, "cert.pem"))
	if err != nil {
		return err
	}

	userConfig.Mode = "https"
	if len(userConfig.Domains) > 0 {
		userConfig.Mode = "acme"
	}

	valid := false
	if userConfig.Key != "" && userConfig.Cert != "" {
		valid = true
	} else if userConfig.Key == "" && userConfig.Cert == "" {
		valid = true
	}

	if !valid {
		return fmt.Errorf("invalid SSL configuration found, please set cert/key, cert/key/cacerts, cacerts only, or none")
	}

	return nil
}

func readPEM(path string) (string, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return "", nil
	}

	return string(content), nil
}

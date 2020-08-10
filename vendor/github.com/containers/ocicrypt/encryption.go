/*
   Copyright The ocicrypt Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package ocicrypt

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/containers/ocicrypt/blockcipher"
	"github.com/containers/ocicrypt/config"
	"github.com/containers/ocicrypt/keywrap"
	"github.com/containers/ocicrypt/keywrap/jwe"
	"github.com/containers/ocicrypt/keywrap/pgp"
	"github.com/containers/ocicrypt/keywrap/pkcs7"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// EncryptLayerFinalizer is a finalizer run to return the annotations to set for
// the encrypted layer
type EncryptLayerFinalizer func() (map[string]string, error)

func init() {
	keyWrappers = make(map[string]keywrap.KeyWrapper)
	keyWrapperAnnotations = make(map[string]string)
	RegisterKeyWrapper("pgp", pgp.NewKeyWrapper())
	RegisterKeyWrapper("jwe", jwe.NewKeyWrapper())
	RegisterKeyWrapper("pkcs7", pkcs7.NewKeyWrapper())
}

var keyWrappers map[string]keywrap.KeyWrapper
var keyWrapperAnnotations map[string]string

// RegisterKeyWrapper allows to register key wrappers by their encryption scheme
func RegisterKeyWrapper(scheme string, iface keywrap.KeyWrapper) {
	keyWrappers[scheme] = iface
	keyWrapperAnnotations[iface.GetAnnotationID()] = scheme
}

// GetKeyWrapper looks up the encryptor interface given an encryption scheme (gpg, jwe)
func GetKeyWrapper(scheme string) keywrap.KeyWrapper {
	return keyWrappers[scheme]
}

// GetWrappedKeysMap returns a map of wrappedKeys as values in a
// map with the encryption scheme(s) as the key(s)
func GetWrappedKeysMap(desc ocispec.Descriptor) map[string]string {
	wrappedKeysMap := make(map[string]string)

	for annotationsID, scheme := range keyWrapperAnnotations {
		if annotation, ok := desc.Annotations[annotationsID]; ok {
			wrappedKeysMap[scheme] = annotation
		}
	}
	return wrappedKeysMap
}

// EncryptLayer encrypts the layer by running one encryptor after the other
func EncryptLayer(ec *config.EncryptConfig, encOrPlainLayerReader io.Reader, desc ocispec.Descriptor) (io.Reader, EncryptLayerFinalizer, error) {
	var (
		encLayerReader io.Reader
		err            error
		encrypted      bool
		bcFin          blockcipher.Finalizer
		privOptsData   []byte
		pubOptsData    []byte
	)

	if ec == nil {
		return nil, nil, errors.New("EncryptConfig must not be nil")
	}

	for annotationsID := range keyWrapperAnnotations {
		annotation := desc.Annotations[annotationsID]
		if annotation != "" {
			privOptsData, err = decryptLayerKeyOptsData(&ec.DecryptConfig, desc)
			if err != nil {
				return nil, nil, err
			}
			pubOptsData, err = getLayerPubOpts(desc)
			if err != nil {
				return nil, nil, err
			}
			// already encrypted!
			encrypted = true
		}
	}

	if !encrypted {
		encLayerReader, bcFin, err = commonEncryptLayer(encOrPlainLayerReader, desc.Digest, blockcipher.AES256CTR)
		if err != nil {
			return nil, nil, err
		}
	}

	encLayerFinalizer := func() (map[string]string, error) {
		// If layer was already encrypted, bcFin should be nil, use existing optsData
		if bcFin != nil {
			opts, err := bcFin()
			if err != nil {
				return nil, err
			}
			privOptsData, err = json.Marshal(opts.Private)
			if err != nil {
				return nil, errors.Wrapf(err, "could not JSON marshal opts")
			}
			pubOptsData, err = json.Marshal(opts.Public)
			if err != nil {
				return nil, errors.Wrapf(err, "could not JSON marshal opts")
			}
		}

		newAnnotations := make(map[string]string)
		for annotationsID, scheme := range keyWrapperAnnotations {
			b64Annotations := desc.Annotations[annotationsID]
			keywrapper := GetKeyWrapper(scheme)
			b64Annotations, err = preWrapKeys(keywrapper, ec, b64Annotations, privOptsData)
			if err != nil {
				return nil, err
			}
			if b64Annotations != "" {
				newAnnotations[annotationsID] = b64Annotations
			}
		}

		newAnnotations["org.opencontainers.image.enc.pubopts"] = base64.StdEncoding.EncodeToString(pubOptsData)

		if len(newAnnotations) == 0 {
			return nil, errors.New("no encryptor found to handle encryption")
		}

		return newAnnotations, err
	}

	// if nothing was encrypted, we just return encLayer = nil
	return encLayerReader, encLayerFinalizer, err

}

// preWrapKeys calls WrapKeys and handles the base64 encoding and concatenation of the
// annotation data
func preWrapKeys(keywrapper keywrap.KeyWrapper, ec *config.EncryptConfig, b64Annotations string, optsData []byte) (string, error) {
	newAnnotation, err := keywrapper.WrapKeys(ec, optsData)
	if err != nil || len(newAnnotation) == 0 {
		return b64Annotations, err
	}
	b64newAnnotation := base64.StdEncoding.EncodeToString(newAnnotation)
	if b64Annotations == "" {
		return b64newAnnotation, nil
	}
	return b64Annotations + "," + b64newAnnotation, nil
}

// DecryptLayer decrypts a layer trying one keywrap.KeyWrapper after the other to see whether it
// can apply the provided private key
// If unwrapOnly is set we will only try to decrypt the layer encryption key and return
func DecryptLayer(dc *config.DecryptConfig, encLayerReader io.Reader, desc ocispec.Descriptor, unwrapOnly bool) (io.Reader, digest.Digest, error) {
	if dc == nil {
		return nil, "", errors.New("DecryptConfig must not be nil")
	}
	privOptsData, err := decryptLayerKeyOptsData(dc, desc)
	if err != nil || unwrapOnly {
		return nil, "", err
	}

	var pubOptsData []byte
	pubOptsData, err = getLayerPubOpts(desc)
	if err != nil {
		return nil, "", err
	}

	return commonDecryptLayer(encLayerReader, privOptsData, pubOptsData)
}

func decryptLayerKeyOptsData(dc *config.DecryptConfig, desc ocispec.Descriptor) ([]byte, error) {
	privKeyGiven := false
	for annotationsID, scheme := range keyWrapperAnnotations {
		b64Annotation := desc.Annotations[annotationsID]
		if b64Annotation != "" {
			keywrapper := GetKeyWrapper(scheme)

			if keywrapper.NoPossibleKeys(dc.Parameters) {
				continue
			}

			if len(keywrapper.GetPrivateKeys(dc.Parameters)) > 0 {
				privKeyGiven = true
			}

			optsData, err := preUnwrapKey(keywrapper, dc, b64Annotation)
			if err != nil {
				// try next keywrap.KeyWrapper
				continue
			}
			if optsData == nil {
				// try next keywrap.KeyWrapper
				continue
			}
			return optsData, nil
		}
	}
	if !privKeyGiven {
		return nil, errors.New("missing private key needed for decryption")
	}
	return nil, errors.Errorf("no suitable key unwrapper found or none of the private keys could be used for decryption")
}

func getLayerPubOpts(desc ocispec.Descriptor) ([]byte, error) {
	pubOptsString := desc.Annotations["org.opencontainers.image.enc.pubopts"]
	if pubOptsString == "" {
		return json.Marshal(blockcipher.PublicLayerBlockCipherOptions{})
	}
	return base64.StdEncoding.DecodeString(pubOptsString)
}

// preUnwrapKey decodes the comma separated base64 strings and calls the Unwrap function
// of the given keywrapper with it and returns the result in case the Unwrap functions
// does not return an error. If all attempts fail, an error is returned.
func preUnwrapKey(keywrapper keywrap.KeyWrapper, dc *config.DecryptConfig, b64Annotations string) ([]byte, error) {
	if b64Annotations == "" {
		return nil, nil
	}
	for _, b64Annotation := range strings.Split(b64Annotations, ",") {
		annotation, err := base64.StdEncoding.DecodeString(b64Annotation)
		if err != nil {
			return nil, errors.New("could not base64 decode the annotation")
		}
		optsData, err := keywrapper.UnwrapKey(dc, annotation)
		if err != nil {
			continue
		}
		return optsData, nil
	}
	return nil, errors.New("no suitable key found for decrypting layer key")
}

// commonEncryptLayer is a function to encrypt the plain layer using a new random
// symmetric key and return the LayerBlockCipherHandler's JSON in string form for
// later use during decryption
func commonEncryptLayer(plainLayerReader io.Reader, d digest.Digest, typ blockcipher.LayerCipherType) (io.Reader, blockcipher.Finalizer, error) {
	lbch, err := blockcipher.NewLayerBlockCipherHandler()
	if err != nil {
		return nil, nil, err
	}

	encLayerReader, bcFin, err := lbch.Encrypt(plainLayerReader, typ)
	if err != nil {
		return nil, nil, err
	}

	newBcFin := func() (blockcipher.LayerBlockCipherOptions, error) {
		lbco, err := bcFin()
		if err != nil {
			return blockcipher.LayerBlockCipherOptions{}, err
		}
		lbco.Private.Digest = d
		return lbco, nil
	}

	return encLayerReader, newBcFin, err
}

// commonDecryptLayer decrypts an encrypted layer previously encrypted with commonEncryptLayer
// by passing along the optsData
func commonDecryptLayer(encLayerReader io.Reader, privOptsData []byte, pubOptsData []byte) (io.Reader, digest.Digest, error) {
	privOpts := blockcipher.PrivateLayerBlockCipherOptions{}
	err := json.Unmarshal(privOptsData, &privOpts)
	if err != nil {
		return nil, "", errors.Wrapf(err, "could not JSON unmarshal privOptsData")
	}

	lbch, err := blockcipher.NewLayerBlockCipherHandler()
	if err != nil {
		return nil, "", err
	}

	pubOpts := blockcipher.PublicLayerBlockCipherOptions{}
	if len(pubOptsData) > 0 {
		err := json.Unmarshal(pubOptsData, &pubOpts)
		if err != nil {
			return nil, "", errors.Wrapf(err, "could not JSON unmarshal pubOptsData")
		}
	}

	opts := blockcipher.LayerBlockCipherOptions{
		Private: privOpts,
		Public:  pubOpts,
	}

	plainLayerReader, opts, err := lbch.Decrypt(encLayerReader, opts)
	if err != nil {
		return nil, "", err
	}

	return plainLayerReader, opts.Private.Digest, nil
}

// FilterOutAnnotations filters out the annotations belonging to the image encryption 'namespace'
// and returns a map with those taken out
func FilterOutAnnotations(annotations map[string]string) map[string]string {
	a := make(map[string]string)
	if len(annotations) > 0 {
		for k, v := range annotations {
			if strings.HasPrefix(k, "org.opencontainers.image.enc.") {
				continue
			}
			a[k] = v
		}
	}
	return a
}

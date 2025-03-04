package kubeadm

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pkgerrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	bootstrapsecretutil "k8s.io/cluster-bootstrap/util/secrets"
)

// kubeadm bootstrap token utilities cribbed from:
// https://github.com/kubernetes/kubernetes/blob/v1.25.4/cmd/kubeadm/app/apis/bootstraptoken/v1/utils.go
// Copying these instead of importing from kubeadm saves about 4mb of binary size.

// String returns the string representation of the BootstrapTokenString
func (bts BootstrapTokenString) String() string {
	if len(bts.ID) > 0 && len(bts.Secret) > 0 {
		return bootstraputil.TokenFromIDAndSecret(bts.ID, bts.Secret)
	}
	return ""
}

// NewBootstrapTokenString converts the given Bootstrap Token as a string
// to the BootstrapTokenString object used for serialization/deserialization
// and internal usage. It also automatically validates that the given token
// is of the right format
func NewBootstrapTokenString(token string) (*BootstrapTokenString, error) {
	substrs := bootstraputil.BootstrapTokenRegexp.FindStringSubmatch(token)
	if len(substrs) != 3 {
		return nil, fmt.Errorf("the bootstrap token %q was not of the form %q", token, bootstrapapi.BootstrapTokenPattern)
	}

	return &BootstrapTokenString{ID: substrs[1], Secret: substrs[2]}, nil
}

// NewBootstrapTokenStringFromIDAndSecret is a wrapper around NewBootstrapTokenString
// that allows the caller to specify the ID and Secret separately
func NewBootstrapTokenStringFromIDAndSecret(id, secret string) (*BootstrapTokenString, error) {
	return NewBootstrapTokenString(bootstraputil.TokenFromIDAndSecret(id, secret))
}

// BootstrapTokenToSecret converts the given BootstrapToken object to its Secret representation that
// may be submitted to the API Server in order to be stored.
func BootstrapTokenToSecret(bt *BootstrapToken) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstraputil.BootstrapTokenSecretName(bt.Token.ID),
			Namespace: metav1.NamespaceSystem,
		},
		Type: v1.SecretType(bootstrapapi.SecretTypeBootstrapToken),
		Data: encodeTokenSecretData(bt, time.Now()),
	}
}

// encodeTokenSecretData takes the token discovery object and an optional duration and returns the .Data for the Secret
// now is passed in order to be able to used in unit testing
func encodeTokenSecretData(token *BootstrapToken, now time.Time) map[string][]byte {
	data := map[string][]byte{
		bootstrapapi.BootstrapTokenIDKey:     []byte(token.Token.ID),
		bootstrapapi.BootstrapTokenSecretKey: []byte(token.Token.Secret),
	}

	if len(token.Description) > 0 {
		data[bootstrapapi.BootstrapTokenDescriptionKey] = []byte(token.Description)
	}

	// If for some strange reason both token.TTL and token.Expires would be set
	// (they are mutually exclusive in validation so this shouldn't be the case),
	// token.Expires has higher priority, as can be seen in the logic here.
	if token.Expires != nil {
		// Format the expiration date accordingly
		// TODO: This maybe should be a helper function in bootstraputil?
		expirationString := token.Expires.Time.UTC().Format(time.RFC3339)
		data[bootstrapapi.BootstrapTokenExpirationKey] = []byte(expirationString)

	} else if token.TTL != nil && token.TTL.Duration > 0 {
		// Only if .Expires is unset, TTL might have an effect
		// Get the current time, add the specified duration, and format it accordingly
		expirationString := now.Add(token.TTL.Duration).UTC().Format(time.RFC3339)
		data[bootstrapapi.BootstrapTokenExpirationKey] = []byte(expirationString)
	}

	for _, usage := range token.Usages {
		data[bootstrapapi.BootstrapTokenUsagePrefix+usage] = []byte("true")
	}

	if len(token.Groups) > 0 {
		data[bootstrapapi.BootstrapTokenExtraGroupsKey] = []byte(strings.Join(token.Groups, ","))
	}
	return data
}

// BootstrapTokenFromSecret returns a BootstrapToken object from the given Secret
func BootstrapTokenFromSecret(secret *v1.Secret) (*BootstrapToken, error) {
	// Get the Token ID field from the Secret data
	tokenID := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenIDKey)
	if len(tokenID) == 0 {
		return nil, fmt.Errorf("bootstrap Token Secret has no token-id data: %s", secret.Name)
	}

	// Enforce the right naming convention
	if secret.Name != bootstraputil.BootstrapTokenSecretName(tokenID) {
		return nil, fmt.Errorf("bootstrap token name is not of the form '%s(token-id)'. Actual: %q. Expected: %q",
			bootstrapapi.BootstrapTokenSecretPrefix, secret.Name, bootstraputil.BootstrapTokenSecretName(tokenID))
	}

	tokenSecret := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenSecretKey)
	if len(tokenSecret) == 0 {
		return nil, fmt.Errorf("bootstrap Token Secret has no token-secret data: %s", secret.Name)
	}

	// Create the BootstrapTokenString object based on the ID and Secret
	bts, err := NewBootstrapTokenStringFromIDAndSecret(tokenID, tokenSecret)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "bootstrap Token Secret is invalid and couldn't be parsed")
	}

	// Get the description (if any) from the Secret
	description := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenDescriptionKey)

	// Expiration time is optional, if not specified this implies the token
	// never expires.
	secretExpiration := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenExpirationKey)
	var expires *metav1.Time
	if len(secretExpiration) > 0 {
		expTime, err := time.Parse(time.RFC3339, secretExpiration)
		if err != nil {
			return nil, pkgerrors.WithMessagef(err, "can't parse expiration time of bootstrap token %q", secret.Name)
		}
		expires = &metav1.Time{Time: expTime}
	}

	// Build an usages string slice from the Secret data
	var usages []string
	for k, v := range secret.Data {
		// Skip all fields that don't include this prefix
		if !strings.HasPrefix(k, bootstrapapi.BootstrapTokenUsagePrefix) {
			continue
		}
		// Skip those that don't have this usage set to true
		if string(v) != "true" {
			continue
		}
		usages = append(usages, strings.TrimPrefix(k, bootstrapapi.BootstrapTokenUsagePrefix))
	}
	// Only sort the slice if defined
	if usages != nil {
		sort.Strings(usages)
	}

	// Get the extra groups information from the Secret
	// It's done this way to make .Groups be nil in case there is no items, rather than an
	// empty slice or an empty slice with a "" string only
	var groups []string
	groupsString := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenExtraGroupsKey)
	g := strings.Split(groupsString, ",")
	if len(g) > 0 && len(g[0]) > 0 {
		groups = g
	}

	return &BootstrapToken{
		Token:       bts,
		Description: description,
		Expires:     expires,
		Usages:      usages,
		Groups:      groups,
	}, nil
}

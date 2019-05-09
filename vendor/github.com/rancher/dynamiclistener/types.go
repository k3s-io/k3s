package dynamiclistener

import (
	"net/http"
)

type ListenerConfigStorage interface {
	Set(*ListenerStatus) (*ListenerStatus, error)
	Get() (*ListenerStatus, error)
}

type ServerInterface interface {
	Update(status *ListenerStatus) error
	CACert() (string, error)
}

type UserConfig struct {
	// Required fields

	Handler   http.Handler
	HTTPPort  int
	HTTPSPort int
	CertPath  string

	// Optional fields

	KnownIPs    []string
	Domains     []string
	Mode        string
	NoCACerts   bool
	CACerts     string
	Cert        string
	Key         string
	BindAddress string
}

type ListenerStatus struct {
	Revision       string            `json:"revision,omitempty"`
	CACert         string            `json:"caCert,omitempty"`
	CAKey          string            `json:"caKey,omitempty"`
	GeneratedCerts map[string]string `json:"generatedCerts" norman:"nocreate,noupdate"`
	KnownIPs       map[string]bool   `json:"knownIps" norman:"nocreate,noupdate"`
}

func (l *ListenerStatus) DeepCopyInto(t *ListenerStatus) {
	t.Revision = l.Revision
	t.CACert = l.CACert
	t.CAKey = l.CAKey
	t.GeneratedCerts = copyMap(t.GeneratedCerts)
	t.KnownIPs = map[string]bool{}
	for k, v := range l.KnownIPs {
		t.KnownIPs[k] = v
	}
}

func copyMap(m map[string]string) map[string]string {
	ret := map[string]string{}
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

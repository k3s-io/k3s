package control

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdmissionPlugins(t *testing.T) {
	// default behavior
	args := admissionControlers(nil)
	assert.ElementsMatch(t, []string{"enable-admission-plugins=NodeRestriction"}, args)

	args = admissionControlers([]string{"disable-admission-plugins=NodeRestriction", "enable-admission-plugins=Foo"})
	assert.ElementsMatch(t, []string{"disable-admission-plugins=NodeRestriction", "enable-admission-plugins=Foo"}, args)

	args = admissionControlers([]string{"enable-admission-plugins=Foo,Bar", "enable-admission-plugins=Baz"})
	assert.ElementsMatch(t, []string{"enable-admission-plugins=Foo,Bar", "enable-admission-plugins=Baz", "enable-admission-plugins=NodeRestriction"}, args)
}

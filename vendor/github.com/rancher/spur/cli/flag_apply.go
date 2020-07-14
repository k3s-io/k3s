package cli

import (
	"fmt"
	"io/ioutil"
	"strings"
	"syscall"

	"github.com/rancher/spur/flag"
	"github.com/rancher/spur/generic"
)

// Apply will attempt to apply generic flag values to a flagset
func Apply(f Flag, typ string, set *flag.FlagSet) error {
	name := FlagNames(f)[0]
	value, _ := getFlagValue(f)
	usage, _ := getFlagUsage(f)
	envVars, _ := getFlagEnvVars(f)
	filePath, _ := getFlagFilePath(f)
	// make sure we have a pointer to value (for non-generic values)
	if !generic.IsPtr(value) {
		value, _ = getFlagValuePtr(f)
	}
	destination, _ := getFlagDestination(f)
	// create new destination if not defined
	if destination == nil || generic.ValueOfPtr(destination) == nil {
		destination = generic.New(value)
	}
	// create new value if not defined (for generic flag.Value)
	if value == nil || generic.ValueOfPtr(value) == nil {
		value = generic.New(destination)
	}
	wasSet := false
	// load flags from environment or file
	if val, ok := flagFromEnvOrFile(envVars, filePath); ok {
		newValue := generic.New(value)
		if err := applyValue(newValue, val); err != nil {
			return fmt.Errorf("could not parse %q as %s value for flag %s: %s", val, typ, name, err)
		}
		value = newValue
		wasSet = true
	}
	// copy value to destination
	generic.Set(destination, generic.ValueOfPtr(value))
	dest, ok := destination.(flag.Value)
	if !ok {
		dest = flag.NewGenericValue(destination)
	}
	// for all of the names set the flag variable
	for _, name := range FlagNames(f) {
		set.Var(dest, name, usage)
	}
	// if value is not default mark as needs visit
	if wasSet {
		set.NeedsVisit(name)
	}
	return nil
}

func applyValue(ptr interface{}, val string) error {
	if !generic.IsSlice(ptr) {
		// if we are a slice just return the applied elem
		return applyElem(ptr, val)
	}
	// otherwise create a new slice and apply the split values
	values := generic.Zero(ptr)
	for _, val := range strings.Split(val, ",") {
		value := generic.NewElem(ptr)
		if err := generic.FromString(val, value); err != nil {
			return err
		}
		values = generic.Append(values, generic.ValueOfPtr(value))
	}
	generic.Set(ptr, values)
	return nil
}

func applyElem(ptr interface{}, val string) error {
	if gen, ok := ptr.(flag.Value); ok {
		// if we are a generic flag.Value then apply Set
		return gen.Set(val)
	}
	// otherwise create a new value and convert it
	value := generic.NewElem(ptr)
	if err := generic.FromString(val, value); err != nil {
		return err
	}
	generic.Set(ptr, generic.ValueOfPtr(value))
	return nil
}

func flagFromEnvOrFile(envVars []string, filePath string) (val string, ok bool) {
	for _, envVar := range envVars {
		envVar = strings.TrimSpace(envVar)
		if val, ok := syscall.Getenv(envVar); ok {
			return val, true
		}
	}
	for _, fileVar := range strings.Split(filePath, ",") {
		if data, err := ioutil.ReadFile(fileVar); err == nil {
			return string(data), true
		}
	}
	return "", false
}

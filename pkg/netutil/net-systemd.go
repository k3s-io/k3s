package netutil

import (
	"fmt"
	"os"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/dbus"
)

// CheckForbiddenService assess that the systemd services which break the network are not enabled
func CheckForbiddenService() error {
	forbiddenServices := []string{"nm-cloud-setup.service", "nm-cloud-setup.timer"}

	// Verify if we are running systemd. If not, we assume the forbidden services are not present
	if _, err := os.Stat("/usr/lib/systemd");  err != nil {
		retur nil
	}

	connection, err := systemdDbus.NewSystemdConnection()
	if err != nil {
		return err
	}
	defer connection.Close()

	// Check if forbidden services are enabled
	unitsStatus, err := connection.ListUnitsByNames(forbiddenServices)
	if err != nil {
		// ListUnitsByNames method requires systemd version > 230. If that's the case, we use a slower method
		if strings.Contains(fmt.Sprint(err), "Unknown method 'ListUnitsByNames' or interface 'org.freedesktop.systemd1.Manager'.") {
			unitsStatus, err = connection.ListUnits()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	for _, unit := range unitsStatus {
		for _, service := range forbiddenServices {
			if unit.Name == service {
				return fmt.Errorf("Please check the docs. %s is enabled and will break the network connectivity", service)
			}
		}
	}

	return nil
}

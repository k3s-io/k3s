package util

import (
	"fmt"
	"sort"
	"strings"
)

const hyphens = "--"

// ArgValue returns the value of the first matching arg in the provided list.
func ArgValue(searchArg string, extraArgs []string) string {
	var value string
	for _, unsplitArg := range extraArgs {
		splitArg := strings.SplitN(strings.TrimPrefix(unsplitArg, hyphens), "=", 2)
		if splitArg[0] == searchArg {
			value = splitArg[1]
			// break if we found our value
			break
		}
	}
	return value
}

// GetArgs appends extra arguments to existing arguments with logic to override any default
// arguments whilst also allowing to prefix and suffix default string slice arguments.
func GetArgs(initialArgs map[string]string, extraArgs []string) []string {
	multiArgs := make(map[string][]string)

	for _, unsplitArg := range extraArgs {
		splitArg := strings.SplitN(strings.TrimPrefix(unsplitArg, hyphens), "=", 2)
		arg := splitArg[0]
		value := "true"
		if len(splitArg) > 1 {
			value = splitArg[1]
		}

		// After the first iteration, initial args will be empty when handling
		// duplicate arguments as they will form part of existingValues
		cleanedArg := strings.TrimRight(arg, "-+")
		initialValue, initialValueExists := initialArgs[cleanedArg]
		existingValues, existingValuesFound := multiArgs[cleanedArg]

		newValues := make([]string, 0)
		if strings.HasSuffix(arg, "+") { // Append value to initial args
			if initialValueExists {
				newValues = append(newValues, initialValue)
			}
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
			newValues = append(newValues, value)
		} else if strings.HasSuffix(arg, "-") { // Prepend value to initial args
			newValues = append(newValues, value)
			if initialValueExists {
				newValues = append(newValues, initialValue)
			}
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
		} else { // Append value ignoring initial args
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
			newValues = append(newValues, value)
		}

		delete(initialArgs, cleanedArg)
		multiArgs[cleanedArg] = newValues
	}

	// Add any remaining initial args to the map
	for arg, value := range initialArgs {
		multiArgs[arg] = []string{value}
	}

	// Get args so we can output them sorted whilst preserving the order of
	// repeated keys
	var keys []string
	for arg := range multiArgs {
		keys = append(keys, arg)
	}
	sort.Strings(keys)

	var args []string
	for _, arg := range keys {
		values := multiArgs[arg]
		for _, value := range values {
			cmd := fmt.Sprintf("%s%s=%s", hyphens, strings.TrimPrefix(arg, hyphens), value)
			args = append(args, cmd)
		}
	}

	return args
}

// AddFeatureGate correctly appends a feature gate key pair to the feature gates CLI switch.
func AddFeatureGate(current, toAdd string) string {
	if current == "" {
		return toAdd
	}
	return current + "," + toAdd
}

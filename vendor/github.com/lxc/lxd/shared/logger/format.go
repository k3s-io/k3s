package logger

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// Pretty will attempt to convert any Go structure into a string suitable for logging
func Pretty(input interface{}) string {
	pretty, err := json.MarshalIndent(input, "\t", "\t")
	if err != nil {
		return fmt.Sprintf("%v", input)
	}

	return fmt.Sprintf("\n\t%s", pretty)
}

// GetStack will convert the Go stack into a string suitable for logging
func GetStack() string {
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)

	return fmt.Sprintf("\n\t%s", buf[:n])
}

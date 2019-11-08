package shared

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/lxc/lxd/shared/logger"
)

type Jmap map[string]interface{}

func (m Jmap) GetString(key string) (string, error) {
	if val, ok := m[key]; !ok {
		return "", fmt.Errorf("Response was missing `%s`", key)
	} else if val, ok := val.(string); !ok {
		return "", fmt.Errorf("`%s` was not a string", key)
	} else {
		return val, nil
	}
}

func (m Jmap) GetMap(key string) (Jmap, error) {
	if val, ok := m[key]; !ok {
		return nil, fmt.Errorf("Response was missing `%s`", key)
	} else if val, ok := val.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("`%s` was not a map, got %T", key, m[key])
	} else {
		return val, nil
	}
}

func (m Jmap) GetInt(key string) (int, error) {
	if val, ok := m[key]; !ok {
		return -1, fmt.Errorf("Response was missing `%s`", key)
	} else if val, ok := val.(float64); !ok {
		return -1, fmt.Errorf("`%s` was not an int", key)
	} else {
		return int(val), nil
	}
}

func (m Jmap) GetBool(key string) (bool, error) {
	if val, ok := m[key]; !ok {
		return false, fmt.Errorf("Response was missing `%s`", key)
	} else if val, ok := val.(bool); !ok {
		return false, fmt.Errorf("`%s` was not an int", key)
	} else {
		return val, nil
	}
}

func DebugJson(r *bytes.Buffer) {
	pretty := &bytes.Buffer{}
	if err := json.Indent(pretty, r.Bytes(), "\t", "\t"); err != nil {
		logger.Debugf("error indenting json: %s", err)
		return
	}

	// Print the JSON without the last "\n"
	str := pretty.String()
	logger.Debugf("\n\t%s", str[0:len(str)-1])
}

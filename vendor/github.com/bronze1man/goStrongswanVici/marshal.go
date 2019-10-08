package goStrongswanVici

import (
	"encoding/json"
)

//concrete data type to general data type
// concrete data type like *Version
// general data type include map[string]interface{} []string string
// TODO make it faster
func ConvertToGeneral(concrete interface{}, general interface{}) (err error) {
	b, err := json.Marshal(concrete)
	if err != nil {
		return
	}
	return json.Unmarshal(b, general)
}

// general data type to concrete data type
// concrete data type like *Version
// general data type include map[string]interface{} []string string
// TODO make it faster
func ConvertFromGeneral(general interface{}, concrete interface{}) (err error) {
	b, err := json.Marshal(general)
	if err != nil {
		return
	}
	return json.Unmarshal(b, concrete)
}

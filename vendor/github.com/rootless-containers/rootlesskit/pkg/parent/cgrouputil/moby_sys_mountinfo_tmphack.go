package cgrouputil

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/moby/sys/mountinfo"
)

// mountinfoFSType returns m.Fstype on mountinfo v0.1.3,
// returns m.FSType on mountinfo v0.4.0.
func mountinfoFSType(m *mountinfo.Info) (string, bool) {
	elem := reflect.ValueOf(m).Elem()
	for i := 0; i < elem.NumField(); i++ {
		typeField := elem.Type().Field(i)
		name := typeField.Name
		typ := typeField.Type.String()
		if strings.ToLower(name) == "fstype" && typ == "string" {
			value := elem.Field(i).String()
			return value, true
		}
	}

	return "", false
}

// mountinfoFSTypeFilter is reimplementation of mountinfo.FSTypeFilter.
// Temporary solution for supporting both moby/sys/mountinfo@v0.1.3 and @v0.4.0 .
// Will be removed after downstream projects stop using @v0.1.3 .
func mountinfoFSTypeFilter(fstype ...string) mountinfo.FilterFunc {
	return func(m *mountinfo.Info) (bool, bool) {
		mFSType, ok := mountinfoFSType(m)
		if !ok {
			panic(fmt.Errorf("failed to get Fstype (FSType) of %+v", m))
		}
		for _, t := range fstype {
			if mFSType == t {
				return false, false // don't skeep, keep going
			}
		}
		return true, false // skip, keep going
	}
}

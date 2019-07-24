package config

import (
	"reflect"
	"testing"
)

func TestGetArgsList(t *testing.T) {
	argsMap := map[string]string{
		"aaa": "A",
		"bbb": "B",
		"ccc": "C",
		"ddd": "d",
		"eee": "e",
		"fff": "f",
		"ggg": "g",
		"hhh": "h",
	}
	extraArgs := []string{
		"bbb=BB",
		"ddd=DD",
		"iii=II",
	}
	expected := []string{
		"--aaa=A",
		"--bbb=BB",
		"--ccc=C",
		"--ddd=DD",
		"--eee=e",
		"--fff=f",
		"--ggg=g",
		"--hhh=h",
		"--iii=II",
	}
	actual := GetArgsList(argsMap, extraArgs)
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("got %v\nwant %v", actual, expected)
	}
}

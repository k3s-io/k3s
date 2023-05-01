package util

import (
	"reflect"
	"testing"

	"github.com/urfave/cli"
)

func Test_UnitSplitSliceString(t *testing.T) {
	tests := []struct {
		name string
		arg  cli.StringSlice
		want []string
	}{
		{
			name: "Single Argument",
			arg:  cli.StringSlice{"foo"},
			want: []string{"foo"},
		},
		{
			name: "Repeated Arguments",
			arg:  cli.StringSlice{"foo", "bar", "baz"},
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "Multiple Arguments and Repeated Arguments",
			arg:  cli.StringSlice{"foo,bar", "zoo,clar", "baz"},
			want: []string{"foo", "bar", "zoo", "clar", "baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitStringSlice(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitSliceString() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

package util

import (
	"reflect"
	"testing"

	"github.com/urfave/cli/v2"
)

func Test_UnitSplitSliceString(t *testing.T) {
	tests := []struct {
		name string
		arg  *cli.StringSlice
		want []string
	}{
		{
			name: "Single Argument",
			arg:  cli.NewStringSlice("foo"),
			want: []string{"foo"},
		},
		{
			name: "Repeated Arguments",
			arg:  cli.NewStringSlice("foo", "bar", "baz"),
			want: []string{"foo", "bar", "baz"},
		},
		{
			name: "Multiple Arguments and Repeated Arguments",
			arg:  cli.NewStringSlice("foo,bar", "zoo,clar", "baz"),
			want: []string{"foo", "bar", "zoo", "clar", "baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitStringSlice(tt.arg.Value()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitSliceString() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

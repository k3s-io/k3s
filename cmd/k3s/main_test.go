package main

import "testing"

func Test_UnitFindPreferBundledBin(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "Single argument",
			args: []string{"--prefer-bundled-bin"},
			want: true,
		},
		{
			name: "no argument",
			args: []string{""},
			want: false,
		},
		{
			name: "Argument with equal true",
			args: []string{"--prefer-bundled-bin=true"},
			want: true,
		},
		{
			name: "Argument with equal false",
			args: []string{"--prefer-bundled-bin=false"},
			want: false,
		},
		{
			name: "Multiple arguments with space true",
			args: []string{"--abcd", "--prefer-bundled-bin", "true", "--efgh"},
			want: true,
		},
		{
			name: "Multiple arguments with space false",
			args: []string{"--abcd", "--prefer-bundled-bin", "false", "--efgh"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findPreferBundledBin(tt.args); got != tt.want {
				t.Errorf("findPreferBundledBin() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

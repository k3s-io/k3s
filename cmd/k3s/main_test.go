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
			name: "Argument with equal 1",
			args: []string{"--prefer-bundled-bin=1"},
			want: true,
		},
		{
			name: "Argument with equal 0",
			args: []string{"--prefer-bundled-bin=0"},
			want: false,
		},
		{
			name: "Multiple arguments",
			args: []string{"--abcd", "--prefer-bundled-bin", "--efgh"},
			want: true,
		},
		{
			name: "Repeated arguments",
			args: []string{"--abcd", "--prefer-bundled-bin=false", "--prefer-bundled-bin"},
			want: true,
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

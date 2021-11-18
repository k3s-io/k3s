package util

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli"
)

func TestParseStringSliceToIPs(t *testing.T) {
	tests := []struct {
		name           string
		unparsedIPs    cli.StringSlice
		expectedIPs    []net.IP
		expectedErrMsg string
	}{
		{
			name:        "nil string slice must return no errors",
			unparsedIPs: nil,
			expectedIPs: nil,
		},
		{
			name:        "empty string slice must return no errors",
			unparsedIPs: cli.StringSlice{},
			expectedIPs: nil,
		},
		{
			name:        "single element slice with correct IP must succeed",
			unparsedIPs: cli.StringSlice{"10.10.10.10"},
			expectedIPs: []net.IP{net.ParseIP("10.10.10.10")},
		},
		{
			name:        "single element slice with correct IP list must succeed",
			unparsedIPs: cli.StringSlice{"10.10.10.10,10.10.10.11"},
			expectedIPs: []net.IP{
				net.ParseIP("10.10.10.10"),
				net.ParseIP("10.10.10.11"),
			},
		},
		{
			name:        "multi element slice with correct IP list must succeed",
			unparsedIPs: cli.StringSlice{"10.10.10.10,10.10.10.11", "10.10.10.12,10.10.10.13"},
			expectedIPs: []net.IP{
				net.ParseIP("10.10.10.10"),
				net.ParseIP("10.10.10.11"),
				net.ParseIP("10.10.10.12"),
				net.ParseIP("10.10.10.13"),
			},
		},
		{
			name:           "single element slice with correct IP list with trailing comma must fail",
			unparsedIPs:    cli.StringSlice{"10.10.10.10,"},
			expectedIPs:    nil,
			expectedErrMsg: "invalid ip format ''",
		},
		{
			name:           "single element slice with incorrect IP (overflow) must fail",
			unparsedIPs:    cli.StringSlice{"10.10.10.256"},
			expectedIPs:    nil,
			expectedErrMsg: "invalid ip format '10.10.10.256'",
		},
		{
			name:           "single element slice with incorrect IP (foreign symbols) must fail",
			unparsedIPs:    cli.StringSlice{"xxx.yyy.zzz.www"},
			expectedIPs:    nil,
			expectedErrMsg: "invalid ip format 'xxx.yyy.zzz.www'",
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			ips, err := ParseStringSliceToIPs(tc.unparsedIPs)

			if tc.expectedErrMsg != "" {
				assert.Errorf(t, err, tc.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedIPs, ips)
		})
	}
}

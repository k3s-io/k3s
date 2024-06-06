package config

import (
	"os"
	"testing"
)

func TestIsValidResolvConf(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedResult bool
	}{
		{"Valid ResolvConf", "nameserver 8.8.8.8\nnameserver 2001:4860:4860::8888\n", true},
		{"Invalid ResolvConf", "nameserver 999.999.999.999\nnameserver not.an.ip\n", false},
		{"Wrong Nameserver", "search example.com\n", false},
		{name: "One valid nameserver", fileContent: "test test.com\nnameserver 8.8.8.8", expectedResult: true},
		{"Non GlobalUnicast", "nameserver ::1\nnameserver 169.254.0.1\nnameserver fe80::1\n", false},
		{"Empty File", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "resolv.conf")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.fileContent); err != nil {
				t.Errorf("Error writing to file: %v with content: %v", tmpfile.Name(), tt.fileContent)
			}

			res := isValidResolvConf(tmpfile.Name())
			if res != tt.expectedResult {
				t.Errorf("isValidResolvConf(%s) = %v; want %v", tt.name, res, tt.expectedResult)
			}
		})
	}
}

package config

import (
	"os"
	"testing"
)

func Test_isValidResolvConf(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedResult bool
	}{
		{name: "Valid ResolvConf", fileContent: "nameserver 8.8.8.8\nnameserver 2001:4860:4860::8888\n", expectedResult: true},
		{name: "Invalid ResolvConf", fileContent: "nameserver 999.999.999.999\nnameserver not.an.ip\n", expectedResult: false},
		{name: "Wrong Nameserver", fileContent: "search example.com\n", expectedResult: false},
		{name: "One valid nameserver", fileContent: "test test.com\nnameserver 8.8.8.8", expectedResult: true},
		{name: "Non GlobalUnicast", fileContent: "nameserver ::1\nnameserver 169.254.0.1\nnameserver fe80::1\n", expectedResult: false},
		{name: "Empty File", fileContent: "", expectedResult: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "resolv.conf")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.WriteString(tt.fileContent); err != nil {
				t.Errorf("error writing to file: %v with content: %v", tmpfile.Name(), tt.fileContent)
			}

			res := isValidResolvConf(tmpfile.Name())
			if res != tt.expectedResult {
				t.Errorf("isValidResolvConf(%s) = %v; want %v", tt.name, res, tt.expectedResult)
			}
		})
	}
}

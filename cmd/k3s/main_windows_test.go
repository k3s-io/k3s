//go:build windows

package main

import (
	"errors"
	"os/exec"
	"testing"
)

func Test_UnitRunExec(t *testing.T) {
	tests := []struct {
		name             string
		args             []string
		calledAsInternal bool
		wantExitCode     int
	}{
		{
			name:             "internal command drops executable argument",
			args:             []string{"ignored.exe", "/c", "exit 0"},
			calledAsInternal: true,
			wantExitCode:     0,
		},
		{
			name:         "external command receives arguments",
			args:         []string{"/c", "exit 0"},
			wantExitCode: 0,
		},
		{
			name:         "child exit code is returned",
			args:         []string{"/c", "exit 7"},
			wantExitCode: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runExec("cmd.exe", tt.args, tt.calledAsInternal)
			if tt.wantExitCode == 0 {
				if err != nil {
					t.Fatalf("runExec() error = %v, want nil", err)
				}
				return
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("runExec() error = %v, want *exec.ExitError", err)
			}
			if got := exitErr.ExitCode(); got != tt.wantExitCode {
				t.Errorf("runExec() exit code = %d, want %d", got, tt.wantExitCode)
			}
		})
	}
}

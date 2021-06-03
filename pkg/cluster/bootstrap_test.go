package cluster

import (
	"io/ioutil"
	"os"
	"testing"
)

func Test_isDirEmpty(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
		setup   func() string
	}{
		{
			name: "directory is empty",
			args: args{
				name: func() string {
					dir, err := ioutil.TempDir("", "")
					if err != nil {
						t.Fail()
					}

					return dir
				}(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "directory is not empty",
			args: args{
				name: func() string {
					dir, err := ioutil.TempDir("", "")
					if err != nil {
						t.Fail()
					}

					f, err := ioutil.TempFile(dir, "")
					if err != nil {
						t.Fail()
					}
					f.Write([]byte("I'm a little tea pot"))

					return dir
				}(),
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isDirEmpty(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("isDirEmpty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isDirEmpty() = %v, want %v", got, tt.want)
			}
			defer os.RemoveAll(tt.name)
		})
	}
}

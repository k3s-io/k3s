package containerd

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/stretchr/testify/assert"
)

// failingReader fails on the first read, standing in for an I/O error on the
// image list partway through reading it.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("input/output error")
}

func Test_UnitPrePullImages(t *testing.T) {
	tests := []struct {
		name      string
		imageList io.Reader
		wantErr   string
	}{
		{
			name:      "empty list",
			imageList: strings.NewReader(""),
		},
		{
			name:      "blank lines only",
			imageList: strings.NewReader("\n   \n\n"),
		},
		{
			name:      "read fails",
			imageList: failingReader{},
			wantErr:   "input/output error",
		},
		{
			// A line longer than bufio.MaxScanTokenSize, which is what a binary
			// file dropped in the images directory with a .txt name looks like.
			name:      "line too long",
			imageList: strings.NewReader(strings.Repeat("a", 70*1024)),
			wantErr:   "token too long",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No image name is ever parsed in these cases, so the image service
			// and the CRI client are never reached and can be left unconfigured.
			images, err := prePullImages(context.Background(), &containerd.Client{}, nil, tt.imageList)
			assert.Empty(t, images)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

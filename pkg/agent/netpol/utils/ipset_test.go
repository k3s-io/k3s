// Apache License v2.0 (copyright Cloud Native Labs & Rancher Labs)
// - modified from https://github.com/cloudnativelabs/kube-router/blob/73b1b03b32c5755b240f6c077bb097abe3888314/pkg/utils/ipset_test.go

package utils

import "testing"

func Test_buildIPSetRestore(t *testing.T) {
	type args struct {
		ipset *IPSet
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple-restore",
			args: args{
				ipset: &IPSet{Sets: map[string]*Set{
					"foo": {
						Name:    "foo",
						Options: []string{"hash:ip", "yolo", "things", "12345"},
						Entries: []*Entry{
							{Options: []string{"1.2.3.4"}},
						},
					},
					"google-dns-servers": {
						Name:    "google-dns-servers",
						Options: []string{"hash:ip", "lol"},
						Entries: []*Entry{
							{Options: []string{"4.4.4.4"}},
							{Options: []string{"8.8.8.8"}},
						},
					},
					// this one and the one above share the same exact options -- and therefore will reuse the same
					// tmp ipset:
					"more-ip-addresses": {
						Name:    "google-dns-servers",
						Options: []string{"hash:ip", "lol"},
						Entries: []*Entry{
							{Options: []string{"5.5.5.5"}},
							{Options: []string{"6.6.6.6"}},
						},
					},
				}},
			},
			want: "create TMP-7NOTZDOMLXBX6DAJ hash:ip yolo things 12345\n" +
				"flush TMP-7NOTZDOMLXBX6DAJ\n" +
				"add TMP-7NOTZDOMLXBX6DAJ 1.2.3.4\n" +
				"create foo hash:ip yolo things 12345\n" +
				"swap TMP-7NOTZDOMLXBX6DAJ foo\n" +
				"flush TMP-7NOTZDOMLXBX6DAJ\n" +
				"create TMP-XD7BSSQZELS7TP35 hash:ip lol\n" +
				"flush TMP-XD7BSSQZELS7TP35\n" +
				"add TMP-XD7BSSQZELS7TP35 4.4.4.4\n" +
				"add TMP-XD7BSSQZELS7TP35 8.8.8.8\n" +
				"create google-dns-servers hash:ip lol\n" +
				"swap TMP-XD7BSSQZELS7TP35 google-dns-servers\n" +
				"flush TMP-XD7BSSQZELS7TP35\n" +
				"add TMP-XD7BSSQZELS7TP35 5.5.5.5\n" +
				"add TMP-XD7BSSQZELS7TP35 6.6.6.6\n" +
				"create google-dns-servers hash:ip lol\n" +
				"swap TMP-XD7BSSQZELS7TP35 google-dns-servers\n" +
				"flush TMP-XD7BSSQZELS7TP35\n" +
				"destroy TMP-7NOTZDOMLXBX6DAJ\n" +
				"destroy TMP-XD7BSSQZELS7TP35\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildIPSetRestore(tt.args.ipset); got != tt.want {
				t.Errorf("buildIPSetRestore() = %v, want %v", got, tt.want)
			}
		})
	}
}

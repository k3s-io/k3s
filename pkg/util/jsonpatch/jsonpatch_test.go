package jsonpatch

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func Test_JSONPath(t *testing.T) {
	type args struct {
		wantLen       int
		wantPatch     string
		addOperations func(patch PatchBuilder)
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "empty patch",
			args: args{
				wantLen:   0,
				wantPatch: `[]`,
				addOperations: func(patch PatchBuilder) {
				},
			},
		},
		{
			name: "add and remove",
			args: args{
				wantLen: 3,
				wantPatch: `[` +
					`{"op":"add","path":"/a/b","value":1},` +
					`{"op":"add","path":"/b/c","value":["hello","world"]},` +
					`{"op":"remove","path":"/d/e"}` +
					`]`,
				addOperations: func(patch PatchBuilder) {
					patch.Add(1, "a", "b")
					patch.Add([]string{"hello", "world"}, "b", "c")
					patch.Remove("d", "e")
				},
			},
		},
		{
			name: "nested wrapped paths",
			args: args{
				wantLen: 4,
				wantPatch: `[` +
					`{"op":"add","path":"/a","value":{"b":[1]}},` +
					`{"op":"add","path":"/a/b","value":2},` +
					`{"op":"add","path":"/a/b","value":3},` +
					`{"op":"add","path":"/a/b","value":4}` +
					`]`,
				addOperations: func(patch PatchBuilder) {
					v := map[string][]int{
						"b": []int{1},
					}
					patch.Add(v, "a")
					patch.WithPath("a", "b").Add(2).Add(3)
					patch.WithPath("a").WithPath("b").Add(4)
				},
			},
		},
		{
			name: "path escaping",
			args: args{
				wantLen: 4,
				wantPatch: `[` +
					`{"op":"add","path":"/metadata/labels/example.com~1label1","value":"true"},` +
					`{"op":"remove","path":"/metadata/labels/example.com~1label2"},` +
					`{"op":"add","path":"/metadata/annotations/example.com~1annotation1","value":"abc"},` +
					`{"op":"add","path":"/spec/example/~0bar~0","value":"foo"}` +
					`]`,
				addOperations: func(patch PatchBuilder) {
					patch.WithPath("metadata", "labels").Add("true", "example.com/label1").Remove("example.com/label2")
					patch.WithPath("metadata", "annotations").Add("abc", "example.com/annotation1")
					patch.WithPath("spec", "example").Add("foo", "~bar~")
				},
			},
		},
	}
	for _, tt := range tests {
		patch := NewBuilder()
		t.Run(tt.name, func(t *testing.T) {
			tt.args.addOperations(patch)
			if l := patch.Len(); l != tt.args.wantLen {
				t.Errorf("Wanted patch length: %d, got %d", tt.args.wantLen, l)
			}

			res := string(patch.MustMarshal())
			if res != tt.args.wantPatch {
				t.Errorf("Wanted patch: %s\ngot: %s", tt.args.wantPatch, res)
			}
		})
	}
}

func Test_JSONPath_With_Base(t *testing.T) {
	type args struct {
		wantLen       int
		wantPatch     string
		addOperations func(patch PatchBuilder)
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "empty patch",
			args: args{
				wantLen:   0,
				wantPatch: `[]`,
				addOperations: func(patch PatchBuilder) {
				},
			},
		},
		{
			name: "add and remove",
			args: args{
				wantLen: 3,
				wantPatch: `[` +
					`{"op":"add","path":"/root/path/a/b","value":1},` +
					`{"op":"add","path":"/root/path/b/c","value":["hello","world"]},` +
					`{"op":"remove","path":"/root/path/d/e"}` +
					`]`,
				addOperations: func(patch PatchBuilder) {
					patch.Add(1, "a", "b")
					patch.Add([]string{"hello", "world"}, "b", "c")
					patch.Remove("d", "e")
				},
			},
		},
		{
			name: "nested wrapped paths",
			args: args{
				wantLen: 4,
				wantPatch: `[` +
					`{"op":"add","path":"/root/path/a","value":{"b":[1]}},` +
					`{"op":"add","path":"/root/path/a/b","value":2},` +
					`{"op":"add","path":"/root/path/a/b","value":3},` +
					`{"op":"add","path":"/root/path/a/b","value":4}` +
					`]`,
				addOperations: func(patch PatchBuilder) {
					v := map[string][]int{
						"b": []int{1},
					}
					patch.Add(v, "a")
					patch.WithPath("a", "b").Add(2).Add(3)
					patch.WithPath("a").WithPath("b").Add(4)
				},
			},
		},
	}
	for _, tt := range tests {
		patch := NewBuilder("root", "path")
		t.Run(tt.name, func(t *testing.T) {
			tt.args.addOperations(patch)
			if l := patch.Len(); l != tt.args.wantLen {
				t.Errorf("Wanted patch length: %d, got %d", tt.args.wantLen, l)
			}

			res := string(patch.MustMarshal())
			if res != tt.args.wantPatch {
				t.Errorf("Wanted patch: %s\ngot: %s", tt.args.wantPatch, res)
			}
		})
	}
}

func Test_JSONPath_LabelSet(t *testing.T) {
	type args struct {
		wantLen       int
		wantPatch     string
		labelSet      labels.Set
		addOperations func(ls labels.Set, patch PatchBuilder)
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "add and remove",
			args: args{
				wantLen: 2,
				labelSet: labels.Set{
					"example.com/label1": "true",
					"example.com/label2": "1234",
					"example.com/label3": "0000",
				},
				wantPatch: `[` +
					`{"op":"remove","path":"/metadata/labels/example.com~1label1"},` +
					`{"op":"add","path":"/metadata/labels/example.com~1label2","value":"5678"}` +
					`]`,
				addOperations: func(ls labels.Set, patch PatchBuilder) {
					patch = patch.WithPath("metadata", "labels")
					patch.RemoveIfHas(ls, "example.com/label1")
					patch.AddIfNotEqual(ls, "example.com/label2", "5678")
					patch.AddIfNotEqual(ls, "example.com/label3", "0000")
				},
			},
		},
	}
	for _, tt := range tests {
		patch := NewBuilder()
		t.Run(tt.name, func(t *testing.T) {
			tt.args.addOperations(tt.args.labelSet, patch)
			if l := patch.Len(); l != tt.args.wantLen {
				t.Errorf("Wanted patch length: %d, got %d", tt.args.wantLen, l)
			}

			res := string(patch.MustMarshal())
			if res != tt.args.wantPatch {
				t.Errorf("Wanted patch: %s\ngot: %s", tt.args.wantPatch, res)
			}
		})
	}
}

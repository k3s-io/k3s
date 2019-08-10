package bindata

import (
	"regexp"
	"strings"
	"testing"
)

func TestSafeFunctionName(t *testing.T) {
	var knownFuncs = make(map[string]int)
	name1 := safeFunctionName("foo/bar", knownFuncs)
	name2 := safeFunctionName("foo_bar", knownFuncs)
	if name1 == name2 {
		t.Errorf("name collision")
	}
}

func TestFindFiles(t *testing.T) {
	var toc []Asset
	var knownFuncs = make(map[string]int)
	var visitedPaths = make(map[string]bool)
	err := findFiles("testdata/dupname", "testdata/dupname", true, &toc, []*regexp.Regexp{}, knownFuncs, visitedPaths)
	if err != nil {
		t.Errorf("expected to be no error: %+v", err)
	}
	if toc[0].Func == toc[1].Func {
		t.Errorf("name collision")
	}
}

func TestFindFilesWithSymlinks(t *testing.T) {
	var tocSrc []Asset
	var tocTarget []Asset

	var knownFuncs = make(map[string]int)
	var visitedPaths = make(map[string]bool)
	err := findFiles("testdata/symlinkSrc", "testdata/symlinkSrc", true, &tocSrc, []*regexp.Regexp{}, knownFuncs, visitedPaths)
	if err != nil {
		t.Errorf("expected to be no error: %+v", err)
	}

	knownFuncs = make(map[string]int)
	visitedPaths = make(map[string]bool)
	err = findFiles("testdata/symlinkParent", "testdata/symlinkParent", true, &tocTarget, []*regexp.Regexp{}, knownFuncs, visitedPaths)
	if err != nil {
		t.Errorf("expected to be no error: %+v", err)
	}

	if len(tocSrc) != len(tocTarget) {
		t.Errorf("Symlink source and target should have the same number of assets.  Expected %d got %d", len(tocTarget), len(tocSrc))
	} else {
		for i, _ := range tocSrc {
			targetFunc := strings.TrimPrefix(tocTarget[i].Func, "symlinktarget")
			targetFunc = strings.ToLower(targetFunc[:1]) + targetFunc[1:]
			if tocSrc[i].Func != targetFunc {
				t.Errorf("Symlink source and target produced different function lists.  Expected %s to be %s", targetFunc, tocSrc[i].Func)
			}
		}
	}
}

func TestFindFilesWithRecursiveSymlinks(t *testing.T) {
	var toc []Asset

	var knownFuncs = make(map[string]int)
	var visitedPaths = make(map[string]bool)
	err := findFiles("testdata/symlinkRecursiveParent", "testdata/symlinkRecursiveParent", true, &toc, []*regexp.Regexp{}, knownFuncs, visitedPaths)
	if err != nil {
		t.Errorf("expected to be no error: %+v", err)
	}

	if len(toc) != 1 {
		t.Errorf("Only one asset should have been found.  Got %d: %v", len(toc), toc)
	}
}

func TestFindFilesWithSymlinkedFile(t *testing.T) {
	var toc []Asset

	var knownFuncs = make(map[string]int)
	var visitedPaths = make(map[string]bool)
	err := findFiles("testdata/symlinkFile", "testdata/symlinkFile", true, &toc, []*regexp.Regexp{}, knownFuncs, visitedPaths)
	if err != nil {
		t.Errorf("expected to be no error: %+v", err)
	}

	if len(toc) != 1 {
		t.Errorf("Only one asset should have been found.  Got %d: %v", len(toc), toc)
	}
}

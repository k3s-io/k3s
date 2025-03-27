package mock

import (
	"fmt"

	"github.com/onsi/gomega/types"
)

type gomockGomegaMatcher struct {
	gm types.GomegaMatcher
	x  any
}

// GM wraps a gomega matcher for use as a gomock matcher
func GM(gm types.GomegaMatcher) *gomockGomegaMatcher {
	return &gomockGomegaMatcher{gm: gm}
}

func (g *gomockGomegaMatcher) Matches(x any) bool {
	g.x = x
	ok, _ := g.gm.Match(x)
	return ok
}

func (g *gomockGomegaMatcher) String() string {
	if g.x != nil {
		ok, err := g.gm.Match(g.x)
		if err != nil {
			return err.Error()
		}
		if !ok {
			return g.gm.FailureMessage(g.x)
		}
	}
	return fmt.Sprintf("%T", g.gm)
}

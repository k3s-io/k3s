package copyup

type ChildDriver interface {
	CopyUp([]string) ([]string, error)
}

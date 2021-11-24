package dynamiclistener

func OnlyAllow(str string) func(...string) []string {
	return func(s2 ...string) []string {
		for _, s := range s2 {
			if s == str {
				return []string{s}
			}
		}
		return nil
	}
}

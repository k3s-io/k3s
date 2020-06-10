package daemons

func AddFeatureGate(current, new string) string {
	if current == "" {
		return new
	}
	return current + "," + new
}

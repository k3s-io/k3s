package util

// AddFeatureGate correctly appends a feature gate key pair to the feature gates CLI switch.
func AddFeatureGate(current, new string) string {
	if current == "" {
		return new
	}
	return current + "," + new
}

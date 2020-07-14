package cli

func getFlagName(f Flag) (result string, ok bool) {
	if v := flagValue(f).FieldByName("Name"); v.IsValid() {
		return v.Interface().(string), true
	}
	return
}

func getFlagAliases(f Flag) (result []string, ok bool) {
	if v := flagValue(f).FieldByName("Aliases"); v.IsValid() {
		return v.Interface().([]string), true
	}
	return
}

func getFlagEnvVars(f Flag) (result []string, ok bool) {
	if v := flagValue(f).FieldByName("EnvVars"); v.IsValid() {
		return v.Interface().([]string), true
	}
	return
}

func getFlagUsage(f Flag) (result string, ok bool) {
	if v := flagValue(f).FieldByName("Usage"); v.IsValid() {
		return v.Interface().(string), true
	}
	return
}

func getFlagDefaultText(f Flag) (result string, ok bool) {
	if v := flagValue(f).FieldByName("DefaultText"); v.IsValid() {
		return v.Interface().(string), true
	}
	return
}

func getFlagFilePath(f Flag) (result string, ok bool) {
	if v := flagValue(f).FieldByName("FilePath"); v.IsValid() {
		return v.Interface().(string), true
	}
	return
}

func getFlagRequired(f Flag) (result bool, ok bool) {
	if v := flagValue(f).FieldByName("Required"); v.IsValid() {
		return v.Interface().(bool), true
	}
	return
}

func getFlagHidden(f Flag) (result bool, ok bool) {
	if v := flagValue(f).FieldByName("Hidden"); v.IsValid() {
		return v.Interface().(bool), true
	}
	return
}

func getFlagTakesFile(f Flag) (result bool, ok bool) {
	if v := flagValue(f).FieldByName("TakesFile"); v.IsValid() {
		return v.Interface().(bool), true
	}
	return
}

func getFlagSkipAltSrc(f Flag) (result bool, ok bool) {
	if v := flagValue(f).FieldByName("SkipAltSrc"); v.IsValid() {
		return v.Interface().(bool), true
	}
	return
}

func getFlagValue(f Flag) (result interface{}, ok bool) {
	if v := flagValue(f).FieldByName("Value"); v.IsValid() {
		return v.Interface(), true
	}
	return
}

func getFlagValuePtr(f Flag) (result interface{}, ok bool) {
	if v := flagValue(f).FieldByName("Value"); v.IsValid() {
		return v.Addr().Interface(), true
	}
	return
}

func getFlagDestination(f Flag) (result interface{}, ok bool) {
	if v := flagValue(f).FieldByName("Destination"); v.IsValid() {
		return v.Interface(), true
	}
	return
}

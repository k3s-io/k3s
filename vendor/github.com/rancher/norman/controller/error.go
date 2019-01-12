package controller

type ForgetError struct {
	Err    error
	Reason string
}

func (f *ForgetError) Error() string {
	return f.Err.Error()
}

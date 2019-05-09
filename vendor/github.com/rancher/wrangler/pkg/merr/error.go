package merr

import "bytes"


type Errors []error

func (e Errors) Err() error {
	return NewErrors(e...)
}

func (e Errors) Error() string {
	buf := bytes.NewBuffer(nil)
	for _, err := range e {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(err.Error())
	}

	return buf.String()
}

func NewErrors(inErrors ...error) error {
	var errors []error
	for _, err := range inErrors {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	}
	return Errors(errors)
}

package cli

import (
	"fmt"

	"github.com/rancher/spur/flag"
)

// InputSourceContext is an interface used to allow
// other input sources to be implemented as needed.
//
// Source returns an identifier for the input source. In case of file source
// it should return path to the file.
type InputSourceContext interface {
	Source() string
	Get(name string) (interface{}, bool)
}

// ApplyInputSourceValue will attempt to apply an input source to a generic flag
func ApplyInputSourceValue(f Flag, context *Context, isc InputSourceContext) error {
	name := FlagNames(f)[0]
	skipAltSrc, _ := getFlagSkipAltSrc(f)

	if !skipAltSrc && context.flagSet != nil {
		if !context.IsSet(name) {
			// only checks the first name of this flag
			value, ok := isc.Get(name)
			if !ok || value == nil {
				return nil
			}
			// if a generic flag.Value get the string representation
			if v, ok := value.(flag.Value); ok {
				value = v.String()
			}
			// sets the new value from some source
			if err := context.Set(name, value); err != nil {
				return fmt.Errorf("unable to apply input source '%s': %s", isc.Source(), err)
			}
		}
	}
	return nil
}

// ApplyInputSourceValues iterates over all provided flags and executes ApplyInputSourceValue
// on each flag to apply an alternate input source.
func ApplyInputSourceValues(context *Context, inputSourceContext InputSourceContext, flags []Flag) (err error) {
	for _, f := range flags {
		if err = ApplyInputSourceValue(f, context, inputSourceContext); err != nil {
			return err
		}
	}
	return
}

// InitInputSource is used to to setup an InputSourceContext on a Command Before method. It will create a new
// input source based on the func provided with potentially using existing Context values to initialize itself. If there is
// no error it will then apply the new input source to any flags that are supported by the input source
func InitInputSource(flags []Flag, createInputSource func(context *Context) (InputSourceContext, error)) BeforeFunc {
	return func(context *Context) error {
		inputSource, err := createInputSource(context)
		if err != nil {
			return err
		}
		return ApplyInputSourceValues(context, inputSource, flags)
	}
}

// InitAllInputSource is used to to setup an InputSourceContext on a Command Before method. It will create a new
// input source based on the func provided with potentially using existing Context values to initialize itself. If there is
// no error it will then apply the new input source to all flags that are supported by the input source
func InitAllInputSource(createInputSource func(context *Context) (InputSourceContext, error)) BeforeFunc {
	return func(context *Context) error {
		inputSource, err := createInputSource(context)
		if err != nil {
			return err
		}
		return ApplyInputSourceValues(context, inputSource, context.GetFlags())
	}
}

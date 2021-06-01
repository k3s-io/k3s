package util

import "errors"

var ErrCommandNoArgs = errors.New("this command does not take any arguments")
var ErrUnsupportedPlatform = errors.New("unsupported platform")

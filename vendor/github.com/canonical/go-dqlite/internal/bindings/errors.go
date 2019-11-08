package bindings

/*
#include <sqlite3.h>
*/
import "C"

// Error holds information about a SQLite error.
type Error struct {
	Code    int
	Message string
}

func (e Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return C.GoString(C.sqlite3_errstr(C.int(e.Code)))
}

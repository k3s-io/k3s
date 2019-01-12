# FileMutex

FileMutex is similar to `sync.RWMutex`, but also synchronizes across processes.
On Linux, OSX, and other POSIX systems it uses the flock system call. On windows
it uses the LockFileEx and UnlockFileEx system calls.

```go
import (
	"log"
	"github.com/alexflint/go-filemutex"
)

func main() {
	m, err := filemutex.New("/tmp/foo.lock")
	if err != nil {
		log.Fatalln("Directory did not exist or file could not created")
	}

	m.Lock()  // Will block until lock can be acquired

	// Code here is protected by the mutex

	m.Unlock()
}
```

### Installation

    go get github.com/alexflint/go-filemutex

Forked from https://github.com/golang/build/tree/master/cmd/builder/filemutex_*.go

package cancel

import (
	"fmt"
	"net/http"
	"sync"
)

// Canceler tracks a cancelable operation
type Canceler struct {
	reqChCancel map[*http.Request]chan struct{}
	lock        sync.Mutex
}

// NewCanceler returns a new Canceler struct
func NewCanceler() *Canceler {
	c := Canceler{}

	c.lock.Lock()
	c.reqChCancel = make(map[*http.Request]chan struct{})
	c.lock.Unlock()

	return &c
}

// Cancelable indicates whether there are operations that support cancelation
func (c *Canceler) Cancelable() bool {
	c.lock.Lock()
	length := len(c.reqChCancel)
	c.lock.Unlock()

	return length > 0
}

// Cancel will attempt to cancel all ongoing operations
func (c *Canceler) Cancel() error {
	if !c.Cancelable() {
		return fmt.Errorf("This operation can't be canceled at this time")
	}

	c.lock.Lock()
	for req, ch := range c.reqChCancel {
		close(ch)
		delete(c.reqChCancel, req)
	}
	c.lock.Unlock()

	return nil
}

// CancelableDownload performs an http request and allows for it to be canceled at any time
func CancelableDownload(c *Canceler, client *http.Client, req *http.Request) (*http.Response, chan bool, error) {
	chDone := make(chan bool)
	chCancel := make(chan struct{})
	if c != nil {
		c.lock.Lock()
		c.reqChCancel[req] = chCancel
		c.lock.Unlock()
	}
	req.Cancel = chCancel

	go func() {
		<-chDone
		if c != nil {
			c.lock.Lock()
			delete(c.reqChCancel, req)
			c.lock.Unlock()
		}
	}()

	resp, err := client.Do(req)
	return resp, chDone, err
}

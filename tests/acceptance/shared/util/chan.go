package util

import "sync"

func ErrorHandlerChan(wg *sync.WaitGroup, errorChan chan error, task func(chan error)) {
	defer wg.Done()
	task(errorChan)
}

func ErrorProcessorChan(wg *sync.WaitGroup, errorChan chan error, tasks []func(chan error)) {
	for _, task := range tasks {
		wg.Add(1)
		go ErrorHandlerChan(wg, errorChan, task)
	}
}

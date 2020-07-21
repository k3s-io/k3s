package controller

import (
	"context"
)

type hTransactionKey struct{}

type HandlerTransaction struct {
	context.Context
	parent context.Context
	done   chan struct{}
	result bool
}

func (h *HandlerTransaction) do(f func()) {
	if h == nil {
		f()
	} else {
		go func() {
			if h.shouldContinue() {
				f()
			}
		}()
	}
}

func (h *HandlerTransaction) shouldContinue() bool {
	select {
	case <-h.parent.Done():
		return false
	case <-h.done:
		return h.result
	}
}

func (h *HandlerTransaction) Commit() {
	h.result = true
	close(h.done)
}

func (h *HandlerTransaction) Rollback() {
	close(h.done)
}

func NewHandlerTransaction(ctx context.Context) *HandlerTransaction {
	ht := &HandlerTransaction{
		parent: ctx,
		done:   make(chan struct{}),
	}
	ctx = context.WithValue(ctx, hTransactionKey{}, ht)
	ht.Context = ctx
	return ht
}

func getHandlerTransaction(ctx context.Context) *HandlerTransaction {
	v, _ := ctx.Value(hTransactionKey{}).(*HandlerTransaction)
	return v
}

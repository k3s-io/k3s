package server

import "context"

func (l *LimitedServer) dbSize(ctx context.Context) (int64, error) {
	return l.backend.DbSize(ctx)
}

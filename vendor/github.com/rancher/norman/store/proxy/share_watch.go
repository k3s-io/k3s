package proxy

import (
	"github.com/rancher/norman/pkg/broadcast"
	"github.com/rancher/norman/types"
)

func (s *Store) shareWatch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	client, err := s.clientGetter.UnversionedClient(apiContext, s.Context())
	if err != nil {
		return nil, err
	}

	var b *broadcast.Broadcaster
	s.Lock()
	b, ok := s.broadcasters[client]
	if !ok {
		b = &broadcast.Broadcaster{}
		s.broadcasters[client] = b
	}
	s.Unlock()

	return b.Subscribe(apiContext.Request.Context(), func() (chan map[string]interface{}, error) {
		newAPIContext := *apiContext
		newAPIContext.Request = apiContext.Request.WithContext(s.close)
		return s.realWatch(&newAPIContext, schema, &types.QueryOptions{})
	})
}

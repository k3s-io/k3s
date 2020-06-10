/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package events

import (
	"context"

	api "github.com/containerd/containerd/api/services/events/v1"
	apittrpc "github.com/containerd/containerd/api/services/ttrpc/events/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/ttrpc"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.GRPCPlugin,
		ID:   "events",
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			return NewService(ic.Events), nil
		},
	})
}

type service struct {
	ttService *ttrpcService
	events    *exchange.Exchange
}

// NewService returns the GRPC events server
func NewService(events *exchange.Exchange) api.EventsServer {
	return &service{
		ttService: &ttrpcService{
			events: events,
		},
		events: events,
	}
}

func (s *service) Register(server *grpc.Server) error {
	api.RegisterEventsServer(server, s)
	return nil
}

func (s *service) RegisterTTRPC(server *ttrpc.Server) error {
	apittrpc.RegisterEventsService(server, s.ttService)
	return nil
}

func (s *service) Publish(ctx context.Context, r *api.PublishRequest) (*ptypes.Empty, error) {
	if err := s.events.Publish(ctx, r.Topic, r.Event); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &ptypes.Empty{}, nil
}

func (s *service) Forward(ctx context.Context, r *api.ForwardRequest) (*ptypes.Empty, error) {
	if err := s.events.Forward(ctx, fromProto(r.Envelope)); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &ptypes.Empty{}, nil
}

func (s *service) Subscribe(req *api.SubscribeRequest, srv api.Events_SubscribeServer) error {
	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	eventq, errq := s.events.Subscribe(ctx, req.Filters...)
	for {
		select {
		case ev := <-eventq:
			if err := srv.Send(toProto(ev)); err != nil {
				return errors.Wrapf(err, "failed sending event to subscriber")
			}
		case err := <-errq:
			if err != nil {
				return errors.Wrapf(err, "subscription error")
			}

			return nil
		}
	}
}

func toProto(env *events.Envelope) *api.Envelope {
	return &api.Envelope{
		Timestamp: env.Timestamp,
		Namespace: env.Namespace,
		Topic:     env.Topic,
		Event:     env.Event,
	}
}

func fromProto(env *api.Envelope) *events.Envelope {
	return &events.Envelope{
		Timestamp: env.Timestamp,
		Namespace: env.Namespace,
		Topic:     env.Topic,
		Event:     env.Event,
	}
}

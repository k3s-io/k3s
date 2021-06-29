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

	api "github.com/containerd/containerd/api/services/ttrpc/events/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/events/exchange"
	ptypes "github.com/gogo/protobuf/types"
)

type ttrpcService struct {
	events *exchange.Exchange
}

func (s *ttrpcService) Forward(ctx context.Context, r *api.ForwardRequest) (*ptypes.Empty, error) {
	if err := s.events.Forward(ctx, fromTProto(r.Envelope)); err != nil {
		return nil, errdefs.ToGRPC(err)
	}

	return &ptypes.Empty{}, nil
}

func fromTProto(env *api.Envelope) *events.Envelope {
	return &events.Envelope{
		Timestamp: env.Timestamp,
		Namespace: env.Namespace,
		Topic:     env.Topic,
		Event:     env.Event,
	}
}

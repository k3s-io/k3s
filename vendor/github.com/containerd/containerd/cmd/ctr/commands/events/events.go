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
	"encoding/json"
	"fmt"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/typeurl"
	"github.com/urfave/cli"

	// Register grpc event types
	_ "github.com/containerd/containerd/api/events"
)

// Command is the cli command for displaying containerd events
var Command = cli.Command{
	Name:    "events",
	Aliases: []string{"event"},
	Usage:   "display containerd events",
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		eventsClient := client.EventService()
		eventsCh, errCh := eventsClient.Subscribe(ctx, context.Args()...)
		for {
			var e *events.Envelope
			select {
			case e = <-eventsCh:
			case err = <-errCh:
				return err
			}
			if e != nil {
				var out []byte
				if e.Event != nil {
					v, err := typeurl.UnmarshalAny(e.Event)
					if err != nil {
						log.G(ctx).WithError(err).Warn("cannot unmarshal an event from Any")
						continue
					}
					out, err = json.Marshal(v)
					if err != nil {
						log.G(ctx).WithError(err).Warn("cannot marshal Any into JSON")
						continue
					}
				}
				if _, err := fmt.Println(
					e.Timestamp,
					e.Namespace,
					e.Topic,
					string(out),
				); err != nil {
					return err
				}
			}
		}
	},
}

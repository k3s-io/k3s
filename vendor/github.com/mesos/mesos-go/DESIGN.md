## Overview

### Dependencies

Ideals:
- keep it easy for framework writers to integrate w/ mesos-go
- keep it easy for the maintainers of mesos-go

Some guideposts:
- severely limit dependencies on 3rd party libraries
- prefer code generators to compile-time dependencies
- when adding a code generator, add a Makefile target so that it is easily runnable

## Target Audience: Framework Writers

### Developer A: high-level framework writer

**This is a longer-term goal.**
The more immediate need is for a low-level API (see the Developer B section).

Framework-writer goals:
  - wants to provide minimal initial configuration
  - wants to consume a simple event stream
  - wants to inject events into a managed event queue (easy state machine semantics)

One approach is a polling based API, where `Event` objects are produced by some client:

```go
func main() {
...
	c := client.New(scheduler-client-options..) // package scheduler/client
	eventSource := c.ListenWithReconnect()
	ctx := context.TODO()
	for {
		event, err := eventSource.Yield(ctx)
		if err != nil {
			if err == io.EOF {
				// graceful driver termination
				break
			}
			log.Fatalf("FATAL: unrecoverable driver error: %v", err)
		}
		processEvent(event)
	}
	log.Println("scheduler terminating")
}

func processEvent(c *client.Client, ctx context.Context, e events.Event) {
...
	switch e := e.(type) {
	case *mesos.Event:
		... // standard v1 HTTP API Mesos event
	case *client.Event:
		... // high-level driver API event (connected, disconnected, reconnected, ...)
	case *customFramework.Event:
		... // custom framework-generated event, previously annouced via Announcer
	}
...
	if ... {
		go func() {
			// do some time-intensive task and process the result
			// on the driver event queue (easy serialization)
			c.Announce(ctx, &customEvent{...})
		}()
	}
...
	c.AcceptResources(...)
}
```

#### API

##### package events

```go
type (
	// Event objects implement this "marker" interface, which has no meaningful
	// functionality other than to declare that some object is, in fact, an Event.
	// Framework writers may implement their own event objects and for dispatching
	// via some common EventBus.
	Event interface {
		IsEventObject()
	}

	Source interface {
		// Event returns the next available event, blocking until an event becomes available
		// or until the specified context is cancelled. Guaranteed to return a non-nil event
		// if error is nil. Returns an io.EOF error upon graceful termination of the source.
		Yield(Context) (Event, error)
	}

	SourceFunc func(Context) (Event, error) // implements Source interface

	Announcer interface {
		// Announce communicates the occurrence of the event, blocking until the event is
		// successfully announced or until the specified context is cancelled.
		Announce(Context, Event) error
	}

	AnnouncerFunc func(Context, Event) error // implements Announcer interface
)
```

Some possible approaches:
- polling-style API (documented above)
  - framework-writer periodically invokes `client.Event()` to pull the next incoming event from a driver-managed queue
  - pro: more minimal interface than callbacks; API will be more stable over time
  - pro: non-channel API allows scheduler the freedom to re-order the event queue until the last possible moment
  - pro: scheduler can still implement a high-level event-based state machine (just like the chan-based API)
  - con: framework writer is not "forced" to be made aware of new event types (vs. callback-style API)

- channel-based API
  - pro: more minimal interface than callbacks; API will be more stable over time
  - pro: integrates well with select-based event loops
  - pro: scheduler can still implement a high-level event-based state machine (just like the polling-based API)
  - con: chan-based APIs are often not consumed correctly by callers, even when properly documented
  - con: lack of Context on a per-read basis from a Source (less flexible API)
  - con: framework writer is not "forced" to be made aware of new event types (vs. callback-style API)

- callback-style API
  - similar to old mesos-bindings, framework writer submits an `EventHandler`
  - scheduler driver invokes `EventHandler.Handle()` for incoming events
  - pro: a rich interface for EventHandler forces the user to consider all possible event types
  - con: the driver yields total program control to the framework-writer
  - con: new events require EventHandler API changes

### Developer B: low-level framework writer

Goals:
  - wants to manage mesos per-call details (specific codecs, headers, etc)
  - wants to manage entire registration lifecycle (total control over client state)
  - WIP in #211, for example:

```go
func main() {
...
	for {
		err := eventLoop(cli.Do(subscribe, httpcli.Close(true)))
		if err != nil {
			log.Println(err)
		} else {
			log.Println("disconnected")
		}
		log.Println("reconnecting..")
	}
...
}

func eventLoop(events encoding.Decoder, conn io.Closer, err error) error {
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	for err == nil {
		var e scheduler.Event
		if err = events.Decode(&e); err != nil {
			if err == io.EOF {
				err = nil
			}
			continue
		}

		switch e.GetType().Enum() {
		case scheduler.Event_SUBSCRIBED.Enum():
...
```

## References

* [MESOS/scheduler v1 HTTP API](https://github.com/apache/mesos/blob/master/docs/scheduler-http-api.md)
* [MESOS/executor HTTP API Design](https://docs.google.com/document/d/1dFmTrSZXCo5zj8H8SkJ4HT-V0z2YYnEZVV8Fd_-AupM/edit#heading=h.r7o3o3roqg12)

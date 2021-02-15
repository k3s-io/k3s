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

package commonmetrics

import (
	"sync"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// OperationLatencyKey is the key for stargz operation metrics.
	OperationLatencyKey = "operation_duration"

	// Keep namespace as stargz and subsystem as fs.
	namespace = "stargz"
	subsystem = "fs"
)

// Lists all metric labels.
const (
	Mount             = "mount"
	RemoteRegistryGet = "remote_registry_get"
	NodeReaddir       = "node_readdir"
)

var (
	// Buckets for OperationLatency metric in milliseconds.
	latencyBuckets = []float64{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384} // in milliseconds

	// OperationLatency collects operation latency numbers by operation
	// type.
	operationLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      OperationLatencyKey,
			Help:      "Latency in milliseconds of stargz snapshotter operations. Broken down by operation type.",
			Buckets:   latencyBuckets,
		},
		[]string{"operation_type", "layer"},
	)
)

var register sync.Once

// sinceInMilliseconds gets the time since the specified start in milliseconds.
// The division by 1e6 is made to have the milliseconds value as floating point number, since the native method
// .Milliseconds() returns an integer value and you can lost a precision for sub-millisecond values.
func sinceInMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / 1e6
}

// Register metrics. This is always called only once.
func Register() {
	register.Do(func() {
		prometheus.MustRegister(operationLatency)
	})
}

// Wraps the labels attachment as well as calling Observe into a single method.
// Right now we attach the operation and layer sha, so it's possible to see the breakdown for latency
// by operation and individual layers.
func MeasureLatency(operation string, layer digest.Digest, start time.Time) {
	operationLatency.WithLabelValues(operation, layer.String()).Observe(sinceInMilliseconds(start))
}

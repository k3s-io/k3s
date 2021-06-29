/*
Copyright 2017 The Kubernetes Authors.

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

package crictl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"sigs.k8s.io/yaml"
)

const (
	// truncatedImageIDLen is the truncated length of imageID
	truncatedIDLen = 13
)

type listOptions struct {
	// id of container or sandbox
	id string
	// podID of container
	podID string
	// Regular expression pattern to match pod or container
	nameRegexp string
	// Regular expression pattern to match the pod namespace
	podNamespaceRegexp string
	// state of the sandbox
	state string
	// show verbose info for the sandbox
	verbose bool
	// labels are selectors for the sandbox
	labels map[string]string
	// quiet is for listing just container/sandbox/image IDs
	quiet bool
	// output format
	output string
	// all containers
	all bool
	// latest container
	latest bool
	// last n containers
	last int
	// out with truncating the id
	noTrunc bool
	// image used by the container
	image string
}

type execOptions struct {
	// id of container
	id string
	// timeout to stop command
	timeout int64
	// Whether to exec a command in a tty
	tty bool
	// Whether to stream stdin
	stdin bool
	// Command to exec
	cmd []string
}
type attachOptions struct {
	// id of container
	id string
	// Whether the stdin is TTY
	tty bool
	// Whether pass Stdin to container
	stdin bool
}

type portforwardOptions struct {
	// id of sandbox
	id string
	// ports to forward
	ports []string
}

func getSortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func loadContainerConfig(path string) (*pb.ContainerConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.ContainerConfig
	if err := utilyaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func loadPodSandboxConfig(path string) (*pb.PodSandboxConfig, error) {
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var config pb.PodSandboxConfig
	if err := utilyaml.NewYAMLOrJSONDecoder(f, 4096).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config at %s not found", path)
		}
		return nil, err
	}
	return f, nil
}

func getRuntimeClient(context *cli.Context) (pb.RuntimeServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getRuntimeClientConnection(context)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	runtimeClient := pb.NewRuntimeServiceClient(conn)
	return runtimeClient, conn, nil
}

func getImageClient(context *cli.Context) (pb.ImageServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getImageClientConnection(context)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	imageClient := pb.NewImageServiceClient(conn)
	return imageClient, conn, nil
}

func closeConnection(context *cli.Context, conn *grpc.ClientConn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func protobufObjectToJSON(obj proto.Message) (string, error) {
	jsonpbMarshaler := jsonpb.Marshaler{EmitDefaults: true, Indent: "  "}
	marshaledJSON, err := jsonpbMarshaler.MarshalToString(obj)
	if err != nil {
		return "", err
	}
	return marshaledJSON, nil
}

func outputProtobufObjAsJSON(obj proto.Message) error {
	marshaledJSON, err := protobufObjectToJSON(obj)
	if err != nil {
		return err
	}

	fmt.Println(marshaledJSON)
	return nil
}

func outputProtobufObjAsYAML(obj proto.Message) error {
	marshaledJSON, err := protobufObjectToJSON(obj)
	if err != nil {
		return err
	}
	marshaledYAML, err := yaml.JSONToYAML([]byte(marshaledJSON))
	if err != nil {
		return err
	}

	fmt.Println(string(marshaledYAML))
	return nil
}

func outputStatusInfo(status string, info map[string]string, format string, tmplStr string) error {
	// Sort all keys
	keys := []string{}
	for k := range info {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	jsonInfo := "{" + "\"status\":" + status + ","
	for _, k := range keys {
		var res interface{}
		// We attempt to convert key into JSON if possible else use it directly
		if err := json.Unmarshal([]byte(info[k]), &res); err != nil {
			jsonInfo += "\"" + k + "\"" + ":" + "\"" + info[k] + "\","
		} else {
			jsonInfo += "\"" + k + "\"" + ":" + info[k] + ","
		}
	}
	jsonInfo = jsonInfo[:len(jsonInfo)-1]
	jsonInfo += "}"

	switch format {
	case "yaml":
		yamlInfo, err := yaml.JSONToYAML([]byte(jsonInfo))
		if err != nil {
			return err
		}
		fmt.Println(string(yamlInfo))
	case "json":
		var output bytes.Buffer
		if err := json.Indent(&output, []byte(jsonInfo), "", "  "); err != nil {
			return err
		}
		fmt.Println(output.String())
	case "go-template":
		output, err := tmplExecuteRawJSON(tmplStr, jsonInfo)
		if err != nil {
			return err
		}
		fmt.Println(output)
	default:
		fmt.Printf("Don't support %q format\n", format)
	}
	return nil
}

func parseLabelStringSlice(ss []string) (map[string]string, error) {
	labels := make(map[string]string)
	for _, s := range ss {
		pair := strings.Split(s, "=")
		if len(pair) != 2 {
			return nil, fmt.Errorf("incorrectly specified label: %v", s)
		}
		labels[pair[0]] = pair[1]
	}
	return labels, nil
}

// marshalMapInOrder marshalls a map into json in the order of the original
// data structure.
func marshalMapInOrder(m map[string]interface{}, t interface{}) (string, error) {
	s := "{"
	v := reflect.ValueOf(t)
	for i := 0; i < v.Type().NumField(); i++ {
		field := jsonFieldFromTag(v.Type().Field(i).Tag)
		if field == "" || field == "-" {
			continue
		}
		value, err := json.Marshal(m[field])
		if err != nil {
			return "", err
		}
		s += fmt.Sprintf("%q:%s,", field, value)
	}
	s = s[:len(s)-1]
	s += "}"
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// jsonFieldFromTag gets json field name from field tag.
func jsonFieldFromTag(tag reflect.StructTag) string {
	field := strings.Split(tag.Get("json"), ",")[0]
	for _, f := range strings.Split(tag.Get("protobuf"), ",") {
		if !strings.HasPrefix(f, "json=") {
			continue
		}
		field = strings.TrimPrefix(f, "json=")
	}
	return field
}

func getTruncatedID(id, prefix string) string {
	id = strings.TrimPrefix(id, prefix)
	if len(id) > truncatedIDLen {
		id = id[:truncatedIDLen]
	}
	return id
}

func matchesRegex(pattern, target string) bool {
	if pattern == "" {
		return true
	}
	matched, err := regexp.MatchString(pattern, target)
	if err != nil {
		// Assume it's not a match if an error occurs.
		return false
	}
	return matched
}

func matchesImage(imageClient pb.ImageServiceClient, image string, containerImage string) (bool, error) {
	if image == "" {
		return true, nil
	}
	r1, err := ImageStatus(imageClient, image, false)
	if err != nil {
		return false, err
	}
	r2, err := ImageStatus(imageClient, containerImage, false)
	if err != nil {
		return false, err
	}
	if r1.Image == nil || r2.Image == nil {
		// Always return not match if the image doesn't exist.
		return false, nil
	}
	return r1.Image.Id == r2.Image.Id, nil
}

func ctxWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), timeout)
}

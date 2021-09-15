module github.com/containerd/nri

go 1.14

require (
	// when updating containerd, adjust the replace rules accordingly
	github.com/containerd/containerd v1.5.0-beta.3
	github.com/pkg/errors v0.9.1
)

replace (
	github.com/gogo/googleapis => github.com/gogo/googleapis v1.3.2
	github.com/golang/protobuf => github.com/golang/protobuf v1.3.5
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
)

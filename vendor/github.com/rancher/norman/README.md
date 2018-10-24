Norman
========

An API framework for Building [Rancher Style APIs](https://github.com/rancher/api-spec/) backed by K8s CustomResources.

## Building

`make`

## Example

Refer to `examples/`

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/rancher/norman/generator"
	"github.com/rancher/norman/server"
	"github.com/rancher/norman/types"
)

type Foo struct {
	types.Resource
	Name     string `json:"name"`
	Foo      string `json:"foo"`
	SubThing Baz    `json:"subThing"`
}

type Baz struct {
	Name string `json:"name"`
}

var (
	version = types.APIVersion{
		Version: "v1",
		Group:   "io.cattle.core.example",
		Path:    "/example/v1",
	}

	Schemas = types.NewSchemas()
)

func main() {
	if _, err := Schemas.Import(&version, Foo{}); err != nil {
		panic(err)
	}

	server, err := server.NewAPIServer(context.Background(), os.Getenv("KUBECONFIG"), Schemas)
	if err != nil {
		panic(err)
	}

	fmt.Println("Listening on 0.0.0.0:1234")
	http.ListenAndServe("0.0.0.0:1234", server)
}
```


## License
Copyright (c) 2014-2017 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

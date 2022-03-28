/*
Copyright 2014 The Kubernetes Authors.
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

package basicauth

import (
	"context"

	"k8s.io/apiserver/pkg/authentication/authenticator"
)

// Password checks a username and password against a backing authentication
// store and returns a Response or an error if the password could not be
// checked.
type Password interface {
	AuthenticatePassword(ctx context.Context, user, password string) (*authenticator.Response, bool, error)
}

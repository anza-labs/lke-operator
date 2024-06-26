/*
Copyright 2024 lke-operator contributors.

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

package errors

import (
	"errors"
	"net/http"

	"github.com/linode/linodego"
)

var (
	ErrNilSecret         = errors.New("secret is nil")
	ErrTokenMissing      = errors.New("token is missing from secret")
	ErrNoClusterID       = errors.New("no cluster ID")
	ErrInvalidLKEVersion = errors.New("invalid LKE version from API")
	ErrNotReady          = errors.New("not ready")

	ErrLinodeNotFound             = linodego.Error{Code: http.StatusNotFound}
	ErrLinodeResourceNotAvailable = linodego.Error{Code: http.StatusServiceUnavailable}
)

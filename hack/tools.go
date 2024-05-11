//go:build tools
// +build tools

package hack

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/kyverno/chainsaw"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)

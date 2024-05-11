/*
Copyright 2024 anza-labs contributors.

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

package lkeclient

import (
	"context"

	"github.com/linode/linodego"
)

// Client defines a subset of all Linode Client methods required by LKE Operator.
type Client interface {
	ListLKEVersions(ctx context.Context, opts *linodego.ListOptions) ([]linodego.LKEVersion, error)
	ListLKEClusterAPIEndpoints(ctx context.Context, clusterID int, opts *linodego.ListOptions) ([]linodego.LKEClusterAPIEndpoint, error)

	GetLKECluster(ctx context.Context, clusterID int) (*linodego.LKECluster, error)
	CreateLKECluster(ctx context.Context, opts linodego.LKEClusterCreateOptions) (*linodego.LKECluster, error)
	UpdateLKECluster(ctx context.Context, clusterID int, opts linodego.LKEClusterUpdateOptions) (*linodego.LKECluster, error)
	DeleteLKECluster(ctx context.Context, clusterID int) error

	GetLKEClusterKubeconfig(ctx context.Context, clusterID int) (*linodego.LKEClusterKubeconfig, error)
	GetLKEClusterDashboard(ctx context.Context, clusterID int) (*linodego.LKEClusterDashboard, error)

	ListLKENodePools(ctx context.Context, clusterID int, opts *linodego.ListOptions) ([]linodego.LKENodePool, error)
	CreateLKENodePool(ctx context.Context, clusterID int, opts linodego.LKENodePoolCreateOptions) (*linodego.LKENodePool, error)
	UpdateLKENodePool(ctx context.Context, clusterID, poolID int, opts linodego.LKENodePoolUpdateOptions) (*linodego.LKENodePool, error)
	DeleteLKENodePool(ctx context.Context, clusterID, poolID int) error
	DeleteLKENodePoolNode(ctx context.Context, clusterID int, nodeID string) error
}

func New(token, ua string) *linodego.Client {
	linodeClient := linodego.NewClient(nil)

	linodeClient.SetUserAgent(ua)
	linodeClient.SetToken(token)

	return &linodeClient
}

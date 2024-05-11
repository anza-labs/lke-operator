package controller

import (
	"context"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type LKEClusterConfigHandler struct{}

func (h *LKEClusterConfigHandler) OnChange(ctx context.Context, lke *v1alpha1.LKEClusterConfig) (ctrl.Result, error) {
	/*
		1. Get Cluster
		- If Not Exists:
			1. Create Cluster (return requeue)
		- If Exists:
			1. Check state
			2.
	*/
	panic("unimplemented")
}

func (h *LKEClusterConfigHandler) OnDelete(ctx context.Context, lke *v1alpha1.LKEClusterConfig) (ctrl.Result, error) {
	/*
		1. Initiate Deleting cluster (return requeue)
		2. If cluster is deleted, then return OK
	*/
	panic("unimplemented")
}

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

package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	internalerrors "github.com/anza-labs/lke-operator/internal/errors"
	"github.com/anza-labs/lke-operator/internal/lkeclient"
	tracedclient "github.com/anza-labs/lke-operator/internal/lkeclient/traced"
	"github.com/anza-labs/lke-operator/internal/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	TokenKey = "LINODE_TOKEN"
)

// OnChange must be idempotent
func (r *LKEClusterConfigReconciler) OnChange(
	ctx context.Context,
	lke *v1alpha1.LKEClusterConfig,
) (ctrl.Result, error) {
	client, err := r.newLKEClient(ctx, lke.Spec.TokenSecretRef)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create client: %w", err)
	}

	return r.onChange(ctx, client, lke)
}

func (r *LKEClusterConfigReconciler) onChange(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
) (ctrl.Result, error) {
	panic("unimplemented")
}

// OnDelete must be idempotent
func (r *LKEClusterConfigReconciler) OnDelete(
	ctx context.Context,
	lke *v1alpha1.LKEClusterConfig,
) (ctrl.Result, error) {
	client, err := r.newLKEClient(ctx, lke.Spec.TokenSecretRef)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create client: %w", err)
	}

	lke.Status.Phase = mkptr(v1alpha1.PhaseDeleting)

	if err := r.Update(ctx, lke); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set phase %s: %w",
			v1alpha1.PhaseDeleting,
			err,
		)
	}

	return r.onDelete(ctx, client, lke)
}

func (r *LKEClusterConfigReconciler) onDelete(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
) (ctrl.Result, error) {
	/*
		1. Initiate Deleting cluster (return requeue)
		2. If cluster is deleted, then return OK
	*/

	if lke.Status.ClusterID == nil {
		return ctrl.Result{}, internalerrors.ErrNoClusterID
	}

	_, err := client.GetLKECluster(ctx, *lke.Status.ClusterID)
	if err != nil {
		if !errors.Is(err, internalerrors.ErrLinodeNotFound) {
			return ctrl.Result{}, fmt.Errorf("failed to get cluster: %w", err)
		}

		return ctrl.Result{}, nil
	}

	if err = client.DeleteLKECluster(ctx, *lke.Status.ClusterID); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove cluster: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *LKEClusterConfigReconciler) secretFromRef(
	ctx context.Context,
	ref v1alpha1.SecretRef,
) (*corev1.Secret, error) {
	log := log.FromContext(ctx)

	secretRef := types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}

	log.V(8).Info("secret defined, fetching data for client",
		"secret", secretRef)

	secret := new(corev1.Secret)
	if err := r.Get(ctx, secretRef, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *LKEClusterConfigReconciler) newLKEClient(
	ctx context.Context,
	ref v1alpha1.SecretRef,
) (lkeclient.Client, error) {
	secret, err := r.secretFromRef(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	if secret == nil {
		return nil, fmt.Errorf("%w: %s/%s",
			internalerrors.ErrNilSecret,
			ref.Namespace,
			ref.Name,
		)
	}

	encodedToken, ok := secret.Data[TokenKey]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s (key:%q)",
			internalerrors.ErrTokenMissing,
			secret.Namespace,
			secret.Name,
			TokenKey,
		)
	}

	token, err := base64.StdEncoding.DecodeString(string(encodedToken))
	if err != nil {
		return nil, err
	}

	ua := fmt.Sprintf("lke-operator/%s (%s; %s)",
		version.Version,
		version.OS,
		version.Arch,
	)

	return tracedclient.NewClientWithTracing(
		lkeclient.New(string(token), ua),
		"dynamic_lke_traced_client",
	), nil
}

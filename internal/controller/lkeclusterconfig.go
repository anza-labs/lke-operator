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

package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	internalerrors "github.com/anza-labs/lke-operator/internal/errors"
	"github.com/anza-labs/lke-operator/internal/lkeclient"
	tracedlke "github.com/anza-labs/lke-operator/internal/lkeclient/traced"
	"github.com/anza-labs/lke-operator/internal/version"
	"github.com/linode/linodego"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	cluster, err := client.GetLKECluster(ctx, *lke.Status.ClusterID)
	if err != nil {
		if !errors.Is(err, internalerrors.ErrLinodeNotFound) {
			return ctrl.Result{}, fmt.Errorf("failed to get cluster: %w", err)
		}

		return r.onChangeCreate(ctx, client, lke)
	}

	return r.onChangeUpdate(ctx, client, lke, cluster)
}

func (r *LKEClusterConfigReconciler) onChangeCreate(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
) (ctrl.Result, error) {
	opts := linodego.LKEClusterCreateOptions{
		Label:     lke.Name,
		Region:    lke.Spec.Region,
		NodePools: makeNodePools(lke.Spec.NodePools),
	}

	if lke.Spec.HighAvailability != nil {
		opts.ControlPlane = &linodego.LKEClusterControlPlane{
			HighAvailability: *lke.Spec.HighAvailability,
		}
	}

	if lke.Spec.KubernetesVersion == nil || *lke.Spec.KubernetesVersion == "latest" {
		versions, err := client.ListLKEVersions(ctx, nil)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to list LKE versions: %w", err)
		}

		latest, err := getLatestVersion(versions)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get latest LKE versions: %w", err)
		}

		opts.K8sVersion = latest.ID
	} else {
		opts.K8sVersion = *lke.Spec.KubernetesVersion
	}

	var err error

	cluster, err := client.CreateLKECluster(ctx, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster: %w", err)
	}

	lke.Status.Phase = mkptr(v1alpha1.PhaseProvisioning)
	lke.Status.ClusterID = &cluster.ID

	if err := r.Update(ctx, lke); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set phase %s: %w",
			v1alpha1.PhaseProvisioning,
			err,
		)
	}

	return ctrl.Result{Requeue: true}, nil
}

func makeNodePools(nps []v1alpha1.LKENodePool) []linodego.LKENodePoolCreateOptions {
	lkenps := []linodego.LKENodePoolCreateOptions{}

	for _, np := range nps {
		var autoscaler *linodego.LKENodePoolAutoscaler

		if np.Autoscaler != nil {
			autoscaler = &linodego.LKENodePoolAutoscaler{
				Enabled: true,
				Min:     np.Autoscaler.Min,
				Max:     np.Autoscaler.Max,
			}
		}

		lkenps = append(lkenps, linodego.LKENodePoolCreateOptions{
			Count:      np.NodeCount,
			Type:       np.LinodeType,
			Autoscaler: autoscaler,
		})
	}

	return lkenps
}

func getLatestVersion(versions []linodego.LKEVersion) (linodego.LKEVersion, error) {
	latestMajor, latestMinor, _ := getMajorMinor("1.25")

	if len(versions) == 0 {
		return linodego.LKEVersion{}, fmt.Errorf("%w: empty", internalerrors.ErrInvalidLKEVersion)
	}

	for _, ver := range versions {
		major, minor, err := getMajorMinor(ver.ID)
		if err != nil {
			return linodego.LKEVersion{}, err
		}

		if major > latestMajor {
			latestMajor = major
			latestMinor = minor
		} else if major == latestMajor && minor > latestMinor {
			latestMinor = minor
		}
	}

	return linodego.LKEVersion{
		ID: fmt.Sprintf("%d.%d", latestMajor, latestMinor),
	}, nil
}

func getMajorMinor(id string) (int, int, error) {
	split := strings.Split(id, ".")

	if len(split) != 2 {
		return -1, -1, fmt.Errorf("%w: %q",
			internalerrors.ErrInvalidLKEVersion,
			id,
		)
	}

	major, err := strconv.Atoi(split[0])
	if err != nil {
		return -1, -1, fmt.Errorf("major %q conversion failed: %w",
			split[1],
			err,
		)
	}

	minor, err := strconv.Atoi(split[1])
	if err != nil {
		return -1, -1, fmt.Errorf("minor %q conversion failed: %w",
			split[1],
			err,
		)
	}

	return major, minor, nil
}

func (r *LKEClusterConfigReconciler) onChangeUpdate(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
	cluster *linodego.LKECluster,
) (ctrl.Result, error) {
	// Try to get kubeconfig -> if so, it is ready
	kubeconfig, err := client.GetLKEClusterKubeconfig(ctx, cluster.ID)
	if err != nil {
		if !errors.Is(err, internalerrors.ErrLinodeKubeconfigNotAvailable) {
			return ctrl.Result{}, fmt.Errorf("failed to fetch kubeconfig: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	var (
		sc         = r.KubernetesClient.CoreV1().Secrets(lke.Namespace)
		secretName = lke.Name + "-kubeconfig"
	)

	// try to get secret, if not exists, create, else update
	secret, err := sc.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get kubeconfig secret: %w", err)
		}

		if _, err := sc.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: lke.Namespace,
			},
			Data: map[string][]byte{},
		}, metav1.CreateOptions{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create kubeconfig secret: %w", err)
		}

		return ctrl.Result{}, nil
	}

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

	log.V(8).Info("fetching token for client",
		"secret.name", ref.Name,
		"secret.namespace", ref.Namespace)

	secret, err := r.KubernetesClient.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
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

	return tracedlke.NewClientWithTracing(
		lkeclient.New(string(token), ua),
		"dynamic_lke_traced_client",
	), nil
}

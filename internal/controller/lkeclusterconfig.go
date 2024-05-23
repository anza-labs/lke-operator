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
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	internalerrors "github.com/anza-labs/lke-operator/internal/errors"
	"github.com/anza-labs/lke-operator/internal/lkeclient"
	tracedlke "github.com/anza-labs/lke-operator/internal/lkeclient/traced"
	"github.com/anza-labs/lke-operator/internal/resty/logger"
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
	if lke.Status.ClusterID == nil {
		return r.onChangeCreate(ctx, client, lke)
	}

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

	if lke.Annotations != nil {
		if rawTags, ok := lke.Annotations[lkeTagsAnnotation]; ok {
			opts.Tags = extractTags(rawTags)
		}
	}

	if lke.Spec.HighAvailability != nil {
		opts.ControlPlane = &linodego.LKEClusterControlPlaneOptions{
			HighAvailability: lke.Spec.HighAvailability,
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
	lke.Status.NodePoolStatuses = generateNodePoolStatusesFromSpec(lke.Spec.NodePools)

	if err := r.Update(ctx, lke); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set phase %s: %w",
			v1alpha1.PhaseProvisioning,
			err,
		)
	}

	nps, err := client.ListLKENodePools(ctx, cluster.ID, &linodego.ListOptions{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list node pools: %w", err)
	}

	lke.Status.NodePoolStatuses = generateNodePoolStatusesFromAPI(nps)
	if err := r.Update(ctx, lke); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to updates statuses %s: %w",
			v1alpha1.PhaseProvisioning,
			err,
		)
	}

	return ctrl.Result{Requeue: true}, nil
}

func makeNodePools(nps map[string]v1alpha1.LKENodePool) []linodego.LKENodePoolCreateOptions {
	lkenps := []linodego.LKENodePoolCreateOptions{}

	for name, np := range nps {
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
			Tags:       []string{lkeOperatorTag + name},
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

	if major < 0 || minor < 0 {
		return -1, -1, fmt.Errorf("%w: %q",
			internalerrors.ErrInvalidLKEVersion,
			id,
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
	opts := linodego.LKEClusterUpdateOptions{}

	var (
		markUpdating        bool
		destructiveMutation bool
	)

	opts = updateTags(lke, cluster, opts)

	opts, destructiveMutation = updateControlPlane(lke, cluster, opts)
	if destructiveMutation && !markUpdating {
		markUpdating = destructiveMutation
	}

	if markUpdating {
		lke.Status.Phase = mkptr(v1alpha1.PhaseUpdating)
		if err := r.Update(ctx, lke); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
		}
	}

	_, err := client.UpdateLKECluster(ctx, cluster.ID, opts)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update LKE cluster: %w", err)
	}

	err = r.reconcileNodePools(ctx, client, lke, cluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update node pools: %w", err)
	}

	if err := r.saveKubeconfig(ctx, client, lke, cluster); err != nil {
		if errors.Is(err, internalerrors.ErrNotReady) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, err
	}

	if err := clusterReady(ctx, client, cluster); err != nil {
		if errors.Is(err, internalerrors.ErrNotReady) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get cluster readiness: %w", err)
	}

	lke.Status.Phase = mkptr(v1alpha1.PhaseActive)
	if err := r.Update(ctx, lke); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *LKEClusterConfigReconciler) saveKubeconfig(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
	cluster *linodego.LKECluster,
) error {
	kubeconfig, err := client.GetLKEClusterKubeconfig(ctx, cluster.ID)
	if err != nil {
		if !errors.Is(err, internalerrors.ErrLinodeResourceNotAvailable) {
			return fmt.Errorf("failed to fetch kubeconfig: %w", err)
		}

		return internalerrors.ErrNotReady
	}

	var (
		sc         = r.KubernetesClient.CoreV1().Secrets(lke.Namespace)
		secretName = lke.Name + "-kubeconfig"
	)

	// try to get secret, if not exists, create, else update
	secret, err := sc.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return fmt.Errorf("failed to get kubeconfig secret: %w", err)
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: lke.Namespace,
			},
			Data: map[string][]byte{
				kubeconfigKey: []byte(kubeconfig.KubeConfig),
			},
		}

		secret, err = sc.Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create kubeconfig secret: %w", err)
		}
	}

	kc, ok := secret.Data[kubeconfigKey]
	if !ok || !bytes.Equal(kc, []byte(kubeconfig.KubeConfig)) {
		secret.Data[kubeconfigKey] = []byte(kubeconfig.KubeConfig)

		_, err := sc.Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update kubeconfig secret: %w", err)
		}
	}

	return nil
}

func updateTags(
	lke *v1alpha1.LKEClusterConfig,
	cluster *linodego.LKECluster,
	opts linodego.LKEClusterUpdateOptions,
) linodego.LKEClusterUpdateOptions {
	if lke.Annotations == nil {
		lke.Annotations = make(map[string]string)
	}

	rawTags, ok := lke.Annotations[lkeTagsAnnotation]
	if !ok {
		// nothing to do
		return opts
	}

	tags := extractTags(rawTags)

	slices.Sort(tags)
	slices.Sort(cluster.Tags)

	if slices.Equal(tags, cluster.Tags) {
		// nothing to do
		return opts
	}

	opts.Tags = mkptr(tags)

	return opts
}

func updateControlPlane(
	lke *v1alpha1.LKEClusterConfig,
	cluster *linodego.LKECluster,
	opts linodego.LKEClusterUpdateOptions,
) (linodego.LKEClusterUpdateOptions, bool) {
	if lke.Spec.HighAvailability == nil {
		lke.Spec.HighAvailability = mkptr(false)
	}

	opts.ControlPlane = &linodego.LKEClusterControlPlaneOptions{
		HighAvailability: lke.Spec.HighAvailability,
	}

	return opts, cluster.ControlPlane.HighAvailability != *lke.Spec.HighAvailability
}

func (r *LKEClusterConfigReconciler) reconcileNodePools(
	ctx context.Context,
	client lkeclient.Client,
	lke *v1alpha1.LKEClusterConfig,
	cluster *linodego.LKECluster,
) error {
	specStatuses := generateNodePoolStatusesFromSpec(lke.Spec.NodePools)
	statusStatues := lke.Status.NodePoolStatuses

	change, delete, create := compareNodePoolStatuses(specStatuses, statusStatues)

	if len(change) > 0 || len(delete) > 0 || len(create) > 0 {
		lke.Status.Phase = mkptr(v1alpha1.PhaseUpdating)
		if err := r.Update(ctx, lke); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}
	}

	if err := createNodePools(ctx, client, cluster, delete); err != nil {
		return fmt.Errorf("failed to create node pools: %w", err)
	}

	if err := updateNodePools(ctx, client, cluster, delete); err != nil {
		return fmt.Errorf("failed to update node pools: %w", err)
	}

	if err := deleteNodePools(ctx, client, cluster, delete); err != nil {
		return fmt.Errorf("failed to delete node pools: %w", err)
	}

	nps, err := client.ListLKENodePools(ctx, cluster.ID, &linodego.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list node pools: %w", err)
	}

	lke.Status.NodePoolStatuses = generateNodePoolStatusesFromAPI(nps)
	if err := r.Update(ctx, lke); err != nil {
		return fmt.Errorf("failed to update node pools status: %w", err)
	}

	return nil
}

func createNodePools(
	ctx context.Context,
	client lkeclient.Client,
	cluster *linodego.LKECluster,
	statuses map[string]v1alpha1.NodePoolStatus,
) error {
	for name, status := range statuses {
		opts := linodego.LKENodePoolCreateOptions{
			Count: status.NodePoolDetails.NodeCount,
			Type:  status.NodePoolDetails.LinodeType,
			Tags:  []string{lkeOperatorTag + name},
		}

		if _, err := client.CreateLKENodePool(ctx, cluster.ID, opts); err != nil {
			return fmt.Errorf("failed to create node pool: %w", err)
		}
	}

	return nil
}

func updateNodePools(
	ctx context.Context,
	client lkeclient.Client,
	cluster *linodego.LKECluster,
	statuses map[string]v1alpha1.NodePoolStatus,
) error {
	for name, status := range statuses {
		if status.ID != nil {
			if _, err := client.UpdateLKENodePool(
				ctx,
				cluster.ID,
				*status.ID,
				linodego.LKENodePoolUpdateOptions{},
			); err != nil {
				return fmt.Errorf("failed to delete node pool: %w", err)
			}
		} else {
			opts := linodego.LKENodePoolCreateOptions{
				Count: status.NodePoolDetails.NodeCount,
				Type:  status.NodePoolDetails.LinodeType,
				Tags:  []string{lkeOperatorTag + name},
			}

			if _, err := client.CreateLKENodePool(ctx, cluster.ID, opts); err != nil {
				return fmt.Errorf("failed to up-create node pool: %w", err)
			}
		}
	}

	return nil
}

func deleteNodePools(
	ctx context.Context,
	client lkeclient.Client,
	cluster *linodego.LKECluster,
	statuses map[string]v1alpha1.NodePoolStatus,
) error {
	for _, status := range statuses {
		if status.ID != nil {
			if err := client.DeleteLKENodePool(ctx, cluster.ID, *status.ID); err != nil {
				if errors.Is(err, internalerrors.ErrLinodeNotFound) {
					continue
				}

				return fmt.Errorf("failed to delete node pool: %w", err)
			}
		}
	}

	return nil
}

func clusterReady(
	ctx context.Context,
	client lkeclient.Client,
	cluster *linodego.LKECluster,
) error {
	cluster, err := client.GetLKECluster(ctx, cluster.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch LKE cluster: %w", err)
	}

	if cluster.Status != statusReady {
		return internalerrors.ErrNotReady
	}

	nps, err := client.ListLKENodePools(ctx, cluster.ID, &linodego.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch LKE cluster: %w", err)
	}

	for _, np := range nps {
		for _, l := range np.Linodes {
			if l.Status != statusReady {
				return internalerrors.ErrNotReady
			}
		}
	}

	return nil
}

func compareNodePoolStatuses(
	map1 map[string]v1alpha1.NodePoolStatus,
	map2 map[string]v1alpha1.NodePoolStatus,
) (map[string]v1alpha1.NodePoolStatus, map[string]v1alpha1.NodePoolStatus, map[string]v1alpha1.NodePoolStatus) {
	diffValues := make(map[string]v1alpha1.NodePoolStatus)
	missingInMap1 := make(map[string]v1alpha1.NodePoolStatus)
	missingInMap2 := make(map[string]v1alpha1.NodePoolStatus)

	// Check items in map1
	for key, val1 := range map1 {
		val2, exists := map2[key]
		if !exists {
			missingInMap2[key] = val1
		} else if !val1.IsEqual(val2) {
			diffValues[key] = val1
		}
	}

	// Check items in map2
	for key, val2 := range map2 {
		_, exists := map1[key]
		if !exists {
			missingInMap1[key] = val2
		}
	}

	return diffValues, missingInMap1, missingInMap2
}

func generateNodePoolStatusesFromSpec(nps map[string]v1alpha1.LKENodePool) map[string]v1alpha1.NodePoolStatus {
	statuses := map[string]v1alpha1.NodePoolStatus{}

	for name, np := range nps {
		statuses[name] = v1alpha1.NodePoolStatus{
			NodePoolDetails: np,
		}
	}

	return statuses
}

func generateNodePoolStatusesFromAPI(nps []linodego.LKENodePool) map[string]v1alpha1.NodePoolStatus {
	statuses := map[string]v1alpha1.NodePoolStatus{}

	for _, np := range nps {
		name := fmt.Sprintf("unknown-%d", np.ID)

	tagrange:
		for _, tag := range np.Tags {
			if strings.HasPrefix(tag, lkeOperatorTag) {
				split := strings.Split(tag, "=")
				if len(split) != 2 {
					continue tagrange
				}

				name = split[1]
				break tagrange
			}
		}

		status := v1alpha1.NodePoolStatus{
			ID: mkptr(np.ID),
			NodePoolDetails: v1alpha1.LKENodePool{
				NodeCount:  np.Count,
				LinodeType: np.Type,
			},
		}

		if np.Autoscaler.Enabled {
			status.NodePoolDetails.Autoscaler = &v1alpha1.LKENodePoolAutoscaler{
				Min: np.Autoscaler.Min,
				Max: np.Autoscaler.Max,
			}
		}

		statuses[name] = status
	}

	return statuses
}

func stripSpaces(str string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			// if the character is a space, drop it
			return -1

		default:
			return r
		}
	}, str)
}

func split(r rune) bool {
	return r == '\n' || r == ',' || r == '\r'
}

func extractTags(s string) []string {
	arr := []string{}

	for _, v := range strings.FieldsFunc(s, split) {
		v = stripSpaces(v)
		if len(v) > 0 {
			arr = append(arr, v)
		}
	}

	return arr
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
	log := log.FromContext(ctx)

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

	token, ok := secret.Data[TokenKey]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s (key:%q)",
			internalerrors.ErrTokenMissing,
			secret.Namespace,
			secret.Name,
			TokenKey,
		)
	}

	ua := fmt.Sprintf("lke-operator/%s (%s; %s)",
		version.Version,
		version.OS,
		version.Arch,
	)

	client := lkeclient.New(string(token), ua)
	client.SetLogger(logger.Wrap(log))

	return tracedlke.NewClientWithTracing(
		client,
		"dynamic_lke_traced_client",
	), nil
}

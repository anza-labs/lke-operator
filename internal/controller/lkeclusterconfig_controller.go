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
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	lkev1alpha1 "github.com/anza-labs/lke-operator/api/v1alpha1"
)

// LKEClusterConfigReconciler reconciles a LKEClusterConfig object
type LKEClusterConfigReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	KubernetesClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=lke.anza-labs.dev,resources=lkeclusterconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lke.anza-labs.dev,resources=lkeclusterconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lke.anza-labs.dev,resources=lkeclusterconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;delete;get;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LKEClusterConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *LKEClusterConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("reconcile").WithValues("object.namespaced_name", req)

	log.Info("reconciling")

	lke := &v1alpha1.LKEClusterConfig{}
	if err := r.Get(ctx, req.NamespacedName, lke); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("LKEClusterConfig resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		log.Error(err, "failed to get LKEClusterConfig")
		return ctrl.Result{}, err
	}

	if !lke.DeletionTimestamp.IsZero() && lke.Status.ClusterID != nil {
		res, err := r.OnDelete(ctx, lke)
		if err != nil {
			log.Error(err, "on LKE deletion failed")
			return ctrl.Result{}, r.setFailureMessage(ctx, lke, err)
		}

		if !res.Requeue {
			log.Info("removing finalizer",
				"finalizer", lkeFinalizer)

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(lke, lkeFinalizer)
			if err := r.Update(ctx, lke); err != nil {
				log.Error(err, "removing finalizer failed")
				return ctrl.Result{}, err
			}
		}

		return res, nil
	}

	if !controllerutil.ContainsFinalizer(lke, lkeFinalizer) {
		log.Info("adding finalizer",
			"finalizer", lkeFinalizer)

		controllerutil.AddFinalizer(lke, lkeFinalizer)
		if err := r.Update(ctx, lke); err != nil {
			log.Error(err, "adding finalizer failed")
			return ctrl.Result{}, err
		}
	}

	res, err := r.OnChange(ctx, lke)
	if err != nil {
		log.Error(err, "on LKE change failed")
		return ctrl.Result{}, r.setFailureMessage(ctx, lke, err)
	}

	return res, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LKEClusterConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lkev1alpha1.LKEClusterConfig{}).
		Complete(r)
}

func (r *LKEClusterConfigReconciler) setFailureMessage(
	ctx context.Context,
	lke *lkev1alpha1.LKEClusterConfig,
	err error,
) error {
	lke.Status.FailureMessage = mkptr(err.Error())
	if uerr := r.Update(ctx, lke); uerr != nil {
		return errors.Join(err, uerr)
	}

	return err
}

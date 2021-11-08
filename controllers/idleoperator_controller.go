/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"

	CronManagment "github.com/mickaelchau/new-operator/controllers/cron_managment"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/mickaelchau/new-operator/api/v1alpha1"
)

// IdleOperatorReconciler reconciles a IdleOperator object
type IdleOperatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.mickaelchau.fr,resources=idleoperator,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.mickaelchau.fr,resources=idleoperator/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.mickaelchau.fr,resources=idleoperator/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IdleOperator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func watchCRStatus(Deployments []cachev1alpha1.Deploy) {
	fmt.Println("Deployments Status")
	for _, deploy := range Deployments {
		fmt.Println("name:", deploy.Name, "phase:", deploy.Phase, "replicas:", deploy.Size)
	}
}

func (reconciler *IdleOperatorReconciler) getDeploysFromCluster(ctx context.Context, getMatchingDeploys *appsv1.DeploymentList,
	namespace string, label map[string]string) (ctrl.Result, error) {
	options := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(label),
	}
	err := reconciler.List(ctx, getMatchingDeploys, options...)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) buildCRStatus(ctx context.Context, clusterDeploys []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for _, clusterDeploy := range clusterDeploys {
		alreadyInStatus := false
		for i, statusDeploy := range idlingCR.Status.Deployments {
			if statusDeploy.Name == clusterDeploy.ObjectMeta.Name {
				alreadyInStatus = true
				if statusDeploy.Phase != "idle" {
					statusDeploy.Name = clusterDeploy.ObjectMeta.Name
					statusDeploy.Size = *clusterDeploy.Spec.Replicas
					statusDeploy.Phase = "idle"
					idlingCR.Status.Deployments[i] = statusDeploy
				}
			}
		}
		if !alreadyInStatus {
			var newStatusDeploy cachev1alpha1.Deploy
			newStatusDeploy.Name = clusterDeploy.ObjectMeta.Name
			newStatusDeploy.Size = *clusterDeploy.Spec.Replicas
			newStatusDeploy.Phase = "idle"
			idlingCR.Status.Deployments = append(idlingCR.Status.Deployments, newStatusDeploy)
		}
		*clusterDeploy.Spec.Replicas = 0
		err := reconciler.Update(ctx, &clusterDeploy)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = reconciler.Status().Update(ctx, idlingCR)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) givePodsToDeploys(ctx context.Context, clusterDeploys []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for _, clusterDep := range clusterDeploys {
		for i, statusDep := range idlingCR.Status.Deployments {
			if clusterDep.Name == statusDep.Name && statusDep.Phase != "unidle" {
				clusterDep.Spec.Replicas = &statusDep.Size
				statusDep.Phase = "unidle"
				err := reconciler.Update(ctx, &clusterDep)
				if err != nil {
					return ctrl.Result{}, err
				}
				idlingCR.Status.Deployments[i] = statusDep
			}
		}
	}
	err := reconciler.Status().Update(ctx, idlingCR)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) manageIdling(ctx context.Context,
	IdlingCR cachev1alpha1.IdleOperator, idling_type string) (ctrl.Result, error) {
	var idling_spec []cachev1alpha1.IdlingSpecs = IdlingCR.Spec.Idle
	/*
		if idling_type == "idle" {
			idling_spec = IdlingCR.Spec.Idle
		} else {
			idling_spec = IdlingCR.Spec.Unidle
		}
	*/
	for _, depSpecs := range idling_spec {
		for _, label := range depSpecs.MatchingLabels {
			getMatchingDeploys := &appsv1.DeploymentList{}
			res, err := reconciler.getDeploysFromCluster(ctx, getMatchingDeploys, IdlingCR.Namespace, label)
			if err != nil {
				return res, err
			}
			startIdling, err := CronManagment.InTimezoneOrNot(depSpecs.Time, depSpecs.Duration)
			if err != nil {
				return ctrl.Result{}, err
			}
			if startIdling {
				reconciler.buildCRStatus(ctx, getMatchingDeploys.Items, &IdlingCR)
			} else {
				reconciler.givePodsToDeploys(ctx, getMatchingDeploys.Items, &IdlingCR)
			}
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var IdlingCR cachev1alpha1.IdleOperator
	err := reconciler.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &IdlingCR)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	res, err := reconciler.manageIdling(ctx, IdlingCR, "idle")
	if err != nil {
		return res, err
	}
	/*
		res, err = reconciler.manageIdling(ctx, IdlingCR, "unidle")
		if err != nil {
			return res, err
		}
	*/
	watchCRStatus(IdlingCR.Status.Deployments)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (reconciler *IdleOperatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.IdleOperator{}).
		Owns(&appsv1.Deployment{}).
		Complete(reconciler)
}

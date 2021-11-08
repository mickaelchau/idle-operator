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
		fmt.Println("name:", deploy.Name, "phase:", deploy.Phase, "replicas:", deploy.Size, "tested:", deploy.Tested)
	}
}

func (reconciler *IdleOperatorReconciler) getDeploysFromLabel(ctx context.Context, getMatchingDeploys *appsv1.DeploymentList,
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

func (reconciler *IdleOperatorReconciler) getAllClusterDeploys(ctx context.Context, getMatchingDeploys *appsv1.DeploymentList,
	namespace string) (ctrl.Result, error) {
	options := []client.ListOption{
		client.InNamespace(namespace),
	}
	err := reconciler.List(ctx, getMatchingDeploys, options...)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) buildCRStatus(ctx context.Context, clusterDeploys []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator, label map[string]string) (ctrl.Result, error) {
	for _, clusterDeploy := range clusterDeploys {
		alreadyInStatus := false
		for i, statusDeploy := range idlingCR.Status.Deployments {
			if statusDeploy.Name == clusterDeploy.ObjectMeta.Name {
				alreadyInStatus = true
				statusDeploy.Tested = true
				if *clusterDeploy.Spec.Replicas != 0 {
					statusDeploy.Size = *clusterDeploy.Spec.Replicas
				}
				if statusDeploy.Phase == "unidle" {
					statusDeploy.Phase = "idle"
				}
				idlingCR.Status.Deployments[i] = statusDeploy
			}
		}
		if !alreadyInStatus {
			var newStatusDeploy cachev1alpha1.Deploy
			newStatusDeploy.Name = clusterDeploy.ObjectMeta.Name
			newStatusDeploy.Size = *clusterDeploy.Spec.Replicas
			newStatusDeploy.Phase = "idle"
			newStatusDeploy.Tested = true
			idlingCR.Status.Deployments = append(idlingCR.Status.Deployments, newStatusDeploy)
		}
	}
	err := reconciler.Status().Update(ctx, idlingCR)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) givePodsToDeploys(ctx context.Context, clusterDeploys []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for _, depStatus := range idlingCR.Status.Deployments {
		for _, clusterDep := range clusterDeploys {
			if clusterDep.Name == depStatus.Name {
				if !depStatus.Tested {
					clusterDep.Spec.Replicas = &depStatus.Size
				}
				if depStatus.Tested {
					*clusterDep.Spec.Replicas = 0
				}
				err := reconciler.Update(ctx, &clusterDep)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) manageIdling(ctx context.Context,
	IdlingCR cachev1alpha1.IdleOperator, idling_type string) (ctrl.Result, error) {
	for i, statusDep := range IdlingCR.Status.Deployments {
		statusDep.Tested = false
		IdlingCR.Status.Deployments[i] = statusDep
	}
	/*
		err := reconciler.Status().Update(ctx, &IdlingCR)
		if err != nil {
			return ctrl.Result{}, err
		}*/
	idling_spec := IdlingCR.Spec.Idle
	for _, depSpecs := range idling_spec {
		for _, label := range depSpecs.MatchingLabels {
			getMatchingDeploys := &appsv1.DeploymentList{}
			res, err := reconciler.getDeploysFromLabel(ctx, getMatchingDeploys, IdlingCR.Namespace, label)
			if err != nil {
				return res, err
			}
			startIdling, err := CronManagment.InTimezoneOrNot(depSpecs.Time, depSpecs.Duration)
			if err != nil {
				return ctrl.Result{}, err
			}
			if startIdling {
				reconciler.buildCRStatus(ctx, getMatchingDeploys.Items, &IdlingCR, label)
			}
		}
	}
	getAllDeploys := &appsv1.DeploymentList{}
	res, err := reconciler.getAllClusterDeploys(ctx, getAllDeploys, IdlingCR.Namespace)
	if err != nil {
		return res, err
	}
	res, err = reconciler.givePodsToDeploys(ctx, getAllDeploys.Items, &IdlingCR)
	if err != nil {
		return res, err
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

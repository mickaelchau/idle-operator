/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless requestuired by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	cronmanagment "github.com/mickaelchau/new-operator/controllers/cron_managment"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	oplog "sigs.k8s.io/controller-runtime/pkg/log"

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
func watchCRStatus(Deployments []cachev1alpha1.StatusDeployment) {
	logrus.Info("Deployments Status")
	for _, statusDeployment := range Deployments {
		logrus.Infoln("Name =", statusDeployment.Name, "| Size =", statusDeployment.Size, "| Isdled =",
			statusDeployment.IsIdled)
	}
}

func (reconciler *IdleOperatorReconciler) getDeploysFromLabelAndNamespace(context context.Context,
	getMatchingDeploys *appsv1.DeploymentList, label map[string]string, namespace string) (ctrl.Result, error) {
	options := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(label),
	}
	err := reconciler.List(context, getMatchingDeploys, options...)
	if err != nil {
		logrus.Fatalln("Failed to list Deployments (with namespace & label):", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) getAllClusterDeploys(context context.Context,
	getMatchingDeploys *appsv1.DeploymentList) (ctrl.Result, error) {
	err := reconciler.List(context, getMatchingDeploys)
	if err != nil {
		logrus.Fatalln("Failed to list all deployments of cluster:", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) buildCRStatus(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	isStatusChanged := false
	for _, clusterDeployment := range clusterDeployments {
		isalreadyInStatus := false
		for index, statusDeployment := range idlingCR.Status.StatusDeployments {
			if statusDeployment.Name == clusterDeployment.ObjectMeta.Name {
				if !isStatusChanged {
					isStatusChanged = true
				}
				isalreadyInStatus = true
				statusDeployment.IsIdled = true
				if *clusterDeployment.Spec.Replicas != 0 {
					statusDeployment.Size = *clusterDeployment.Spec.Replicas
				}
				idlingCR.Status.StatusDeployments[index] = statusDeployment
			}
		}
		if !isalreadyInStatus {
			if !isStatusChanged {
				isStatusChanged = true
			}
			var newStatusDeployment cachev1alpha1.StatusDeployment
			newStatusDeployment.Name = clusterDeployment.ObjectMeta.Name
			newStatusDeployment.Size = *clusterDeployment.Spec.Replicas
			newStatusDeployment.IsIdled = true
			idlingCR.Status.StatusDeployments = append(idlingCR.Status.StatusDeployments, newStatusDeployment)
		}
	}
	if isStatusChanged {
		err := reconciler.Status().Update(context, idlingCR)
		if err != nil {
			logrus.Fatalln("Update status failed:", err)
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) updateDeployments(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for _, statusDeployment := range idlingCR.Status.StatusDeployments {
		for _, clusterDeployment := range clusterDeployments {
			isclusterDeploymentChanged := false
			if clusterDeployment.Name == statusDeployment.Name {
				if !statusDeployment.IsIdled && *clusterDeployment.Spec.Replicas != statusDeployment.Size {
					clusterDeployment.Spec.Replicas = &statusDeployment.Size
					isclusterDeploymentChanged = true
				}
				if statusDeployment.IsIdled && *clusterDeployment.Spec.Replicas != 0 {
					*clusterDeployment.Spec.Replicas = 0
					isclusterDeploymentChanged = true
				}
				if isclusterDeploymentChanged {
					err := reconciler.Update(context, &clusterDeployment)
					if err != nil {
						logrus.Fatalln("Update deployment failed:", err)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) manageIdling(context context.Context,
	idlingCR cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for index, statusDeployment := range idlingCR.Status.StatusDeployments {
		statusDeployment.IsIdled = false
		idlingCR.Status.StatusDeployments[index] = statusDeployment
	}
	idling_spec := idlingCR.Spec.Idle
	getAllMatchingDeploys := &appsv1.DeploymentList{}
	for _, depSpecs := range idling_spec {
		startIdling, err := cronmanagment.IsInTimezone(depSpecs.Time, depSpecs.Duration)
		if err != nil {
			logrus.Fatalln("Failed to know if idling or not:", err)
			return ctrl.Result{}, err
		}
		if startIdling {
			getMatchingDeploysFromLabel := &appsv1.DeploymentList{}
			for _, label := range depSpecs.MatchingLabels {
				reconcileResult, err := reconciler.getDeploysFromLabelAndNamespace(context, getMatchingDeploysFromLabel,
					label, depSpecs.Namespace)
				logrus.Infoln("label:", label, "is mapped")
				if err != nil {
					logrus.Fatalln("Failed to get deployments:", err)
					return reconcileResult, err
				}
				getAllMatchingDeploys.Items = append(getAllMatchingDeploys.Items, getMatchingDeploysFromLabel.Items...)
			}
		}
	}
	reconciler.buildCRStatus(context, getAllMatchingDeploys.Items, &idlingCR)
	allClusterDeploys := &appsv1.DeploymentList{}
	reconcileResult, err := reconciler.getAllClusterDeploys(context, allClusterDeploys)
	if err != nil {
		logrus.Fatalln("Failed to get deployments:", err)
		return reconcileResult, err
	}
	reconcileResult, err = reconciler.updateDeployments(context, allClusterDeploys.Items, &idlingCR)
	if err != nil {
		logrus.Fatalln("Failed to update cluster deployments:", err)
		return reconcileResult, err
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) Reconcile(context context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = oplog.FromContext(context)
	var idlingCR cachev1alpha1.IdleOperator
	err := reconciler.Get(context, types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}, &idlingCR)
	if errors.IsNotFound(err) {
		logrus.Info("Custom Resource not found in the cluster at the namespace:", request.Namespace, err)
		return ctrl.Result{}, nil
	}
	if err != nil {
		logrus.Fatalln("Failed to get Custom Resource:", err)
		return ctrl.Result{}, err
	}
	reconcileResult, err := reconciler.manageIdling(context, idlingCR)
	if err != nil {
		logrus.Fatalln("Something goes wrong while idling:", err)
		return reconcileResult, err
	}
	watchCRStatus(idlingCR.Status.StatusDeployments)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (reconciler *IdleOperatorReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&cachev1alpha1.IdleOperator{}).
		Owns(&appsv1.Deployment{}).
		Complete(reconciler)
}

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
func watchCRStatus(Deployments []cachev1alpha1.StatusDeployment, namespace string) {
	logrus.Infof("Deployments Status of Namespace: %s", namespace)
	for _, statusDeployment := range Deployments {
		logrus.Infof("Name = %s | Size = %d | Isdled = %t", statusDeployment.Name,
			statusDeployment.Size, statusDeployment.IsIdled)
	}
}

func (reconciler *IdleOperatorReconciler) injectDeploymentsFromLabelAndNamespace(context context.Context,
	matchingDeployments *appsv1.DeploymentList, labels []map[string]string, namespace string) error {
	options := []client.ListOption{
		client.InNamespace(namespace),
	}
	for _, label := range labels {
		options = append(options, client.MatchingLabels(label))
	}
	logrus.Infoln("Injecting deployments with those options:", options)
	err := reconciler.List(context, matchingDeployments, options...)
	if err != nil {
		logrus.Errorln("Failed to list Deployments (with namespace & label):", err)
		return err
	}
	for _, dep := range matchingDeployments.Items {
		logrus.Infoln(dep.Name)
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) injectAllClusterDeployments(context context.Context,
	matchingDeployments *appsv1.DeploymentList) error {
	err := reconciler.List(context, matchingDeployments)
	if err != nil {
		logrus.Errorln("Failed to list all deployments of cluster:", err)
		return err
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) buildCRStatus(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) error {
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
			logrus.Errorln("Update status failed:", err)
			return err
		}
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) updateDeployments(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) (ctrl.Result, error) {
	for _, statusDeployment := range idlingCR.Status.StatusDeployments {
		for _, clusterDeployment := range clusterDeployments {
			isClusterDeploymentChanged := false
			if clusterDeployment.Name == statusDeployment.Name {
				if !statusDeployment.IsIdled && *clusterDeployment.Spec.Replicas != statusDeployment.Size {
					clusterDeployment.Spec.Replicas = &statusDeployment.Size
					isClusterDeploymentChanged = true
				}
				if statusDeployment.IsIdled && *clusterDeployment.Spec.Replicas != 0 {
					*clusterDeployment.Spec.Replicas = 0
					isClusterDeploymentChanged = true
				}
				if isClusterDeploymentChanged {
					err := reconciler.Update(context, &clusterDeployment)
					if err != nil {
						logrus.Errorln("Update deployment failed:", err)
						return ctrl.Result{}, err
					}
				}
			}
		}
	}
	return ctrl.Result{}, nil
}

func (reconciler *IdleOperatorReconciler) manageIdling(context context.Context,
	idlingCR cachev1alpha1.IdleOperator) error {
	for index, statusDeployment := range idlingCR.Status.StatusDeployments {
		statusDeployment.IsIdled = false
		idlingCR.Status.StatusDeployments[index] = statusDeployment
	}
	idlingSpec := idlingCR.Spec.Idle
	allMatchingDeployments := &appsv1.DeploymentList{}
	for _, depSpecs := range idlingSpec {
		isStartIdling, err := cronmanagment.IsInIdleTimezone(depSpecs.Time, depSpecs.Duration)
		if err != nil {
			logrus.Errorln("Failed to know if idling or not:", err)
			continue
		}
		if isStartIdling {
			matchingDeploymentsFromLabel := &appsv1.DeploymentList{}
			err := reconciler.injectDeploymentsFromLabelAndNamespace(context, matchingDeploymentsFromLabel,
				depSpecs.MatchingLabels, idlingCR.Namespace)
			//logrus.Infoln("labels:", depSpecs.MatchingLabels, "are mapped")
			if err != nil {
				logrus.Errorf("Failed to get deployments for labels %s and namespace %s: %s", depSpecs.MatchingLabels,
					idlingCR.Namespace, err)
			}
			allMatchingDeployments.Items = append(allMatchingDeployments.Items, matchingDeploymentsFromLabel.Items...)
		}
	}
	reconciler.buildCRStatus(context, allMatchingDeployments.Items, &idlingCR)
	allClusterDeployments := &appsv1.DeploymentList{}
	err := reconciler.injectAllClusterDeployments(context, allClusterDeployments)
	if err != nil {
		logrus.Errorln("Failed to inject deployments:", err)
		return err
	}
	_, err = reconciler.updateDeployments(context, allClusterDeployments.Items, &idlingCR)
	if err != nil {
		logrus.Errorln("Failed to update cluster deployments:", err)
		return err
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) Reconcile(context context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = oplog.FromContext(context)
	var allNamespaceIdlingCR cachev1alpha1.IdleOperatorList
	err := reconciler.List(context, &allNamespaceIdlingCR)
	if errors.IsNotFound(err) {
		logrus.Infof("Custom Resource not found in the cluster at the namespace %s: %s", request.Namespace, err)
		return ctrl.Result{}, nil
	}
	if err != nil {
		logrus.Errorln("Failed to List all Custom Resource:", err)
		return ctrl.Result{}, err
	}
	for _, idlingCR := range allNamespaceIdlingCR.Items {
		err = reconciler.manageIdling(context, idlingCR)
		if err != nil {
			logrus.Errorln("Something goes wrong while idling:", err)
			return ctrl.Result{}, err
		}
		watchCRStatus(idlingCR.Status.StatusDeployments, idlingCR.Namespace)
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (reconciler *IdleOperatorReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&cachev1alpha1.IdleOperator{}).
		//Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{OwnerType: &cachev1alpha1.IdleOperator{}, IsController: true}).
		Owns(&appsv1.Deployment{}).
		Complete(reconciler)
}

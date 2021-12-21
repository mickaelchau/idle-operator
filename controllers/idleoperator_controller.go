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
	"fmt"

	"github.com/go-logr/logr"
	cachev1alpha1 "github.com/mickaelchau/new-operator/api/v1alpha1"
	cronmanagment "github.com/mickaelchau/new-operator/controllers/cron_managment"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	oplog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

func configureOperatorLogger(request ctrl.Request) logr.Logger {
	opts := zap.Options{} //You can pass flag here
	logger := zap.New(zap.UseFlagOptions(&opts))
	oplog.SetLogger(logger) // told to take this logger by default
	idleoperatorLogger := oplog.Log.
		WithName("idle-operator-reconciler")
	return idleoperatorLogger
}

func watchCRStatus(
	statusDeployments map[string]cachev1alpha1.StatusDeployment,
	namespace string,
	idleoperatorLogger logr.Logger) {
	idleoperatorLogger.Info("Deployments Status")
	for name, statusDeployment := range statusDeployments {
		idleoperatorLogger.Info(
			"List Deployments",
			"Deployment.Name",
			name,
			"Deployment.Replicas",
			fmt.Sprint(statusDeployment.Size))
	}
}

func watchListedDeployments(
	clusterDeployments []appsv1.Deployment,
	namespace string,
	optionsForList []client.ListOption,
	idleoperatorLogger logr.Logger) {
	idleoperatorLogger.Info(
		"Cluster deployments with options",
		"Options",
		optionsForList)
	for index, clusterDeployment := range clusterDeployments {
		idleoperatorLogger.Info(
			"List Deployments",
			"Deployment.order",
			fmt.Sprint(index),
			"Deployment.Name",
			clusterDeployment.Name,
			"Deployment.Replicas",
			fmt.Sprint(*clusterDeployment.Spec.Replicas))
	}
}

func (reconciler *IdleOperatorReconciler) watchClusterDeployments(
	context context.Context,
	namespace string,
	idleoperatorLogger logr.Logger) {
	idleoperatorLogger.Info("Cluster deployments")
	var allNamespaceDeployments appsv1.DeploymentList
	err := reconciler.List(context,
		&allNamespaceDeployments,
		client.InNamespace(namespace))
	if err != nil {
		idleoperatorLogger.Error(
			err,
			"Failed to list all deployments")
	}
	for index, clusterDeployment := range allNamespaceDeployments.Items {
		idleoperatorLogger.Info(
			"List Deployments",
			"Deployment.order",
			fmt.Sprint(index),
			"Deployment.Name",
			clusterDeployment.Name,
			"Deployment.Replicas",
			fmt.Sprint(*clusterDeployment.Spec.Replicas))
	}
}

func (reconciler *IdleOperatorReconciler) injectDeploymentsFromLabelAndNamespace(
	context context.Context,
	labels []map[string]string,
	namespace string,
	matchingDeployments *appsv1.DeploymentList,
	idleoperatorLogger logr.Logger) error {
	optionsForList := []client.ListOption{
		client.InNamespace(namespace),
	}
	for _, label := range labels {
		optionsForList = append(
			optionsForList,
			client.MatchingLabels(label))
	}
	err := reconciler.List(
		context,
		matchingDeployments,
		optionsForList...)
	if err != nil {
		idleoperatorLogger.Error(
			err,
			"Failed to list Deployments (with namespace & label)")
		return err
	}
	watchListedDeployments(
		matchingDeployments.Items,
		namespace,
		optionsForList,
		idleoperatorLogger.V(1))
	return nil
}

func (reconciler *IdleOperatorReconciler) updateClusterDeployment(
	context context.Context,
	clusterDeployment appsv1.Deployment,
	namespace string,
	idleoperatorLogger logr.Logger) {
	err := reconciler.Update(context, &clusterDeployment)
	if err != nil {
		idleoperatorLogger.Error(
			err,
			"Deployment.Name",
			clusterDeployment.ObjectMeta.Name)
	}
	idleoperatorLogger.V(1).Info(
		"UPDATE DEPLOYMENT SUCCEED",
		"Deployment.Name",
		clusterDeployment.ObjectMeta.Name)
}

func (reconciler *IdleOperatorReconciler) matchingDeploymentsChangedToDesiredState(
	context context.Context,
	isStartIdling bool,
	clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator,
	idleoperatorLogger logr.Logger) {
	statusDeployments := idlingCR.Status.StatusDeployments
	for _, clusterDeployment := range clusterDeployments {
		statusDeployment, isInStatus := statusDeployments[clusterDeployment.ObjectMeta.Name]
		if isInStatus && statusDeployment.HasTreated {
			continue
		}
		hasClusterDeploymentChanged := false
		var updatedStatusDeployment cachev1alpha1.StatusDeployment
		updatedStatusDeployment.HasTreated = true
		if isInStatus {
			updatedStatusDeployment.Size = statusDeployment.Size
		}
		if isStartIdling {
			idleoperatorLogger.V(1).Info(
				"MATCH TIMEZONE",
				"deployment.Name",
				clusterDeployment.ObjectMeta.Name)
			if *clusterDeployment.Spec.Replicas != 0 {
				updatedStatusDeployment.Size = *clusterDeployment.Spec.Replicas
				*clusterDeployment.Spec.Replicas = 0
				hasClusterDeploymentChanged = true
			}
		} else {
			idleoperatorLogger.V(1).Info(
				"NOT MATCH TIMEZONE",
				"deployment.Name",
				clusterDeployment.ObjectMeta.Name)
			if *clusterDeployment.Spec.Replicas == 0 &&
				*clusterDeployment.Spec.Replicas != statusDeployment.Size {
				*clusterDeployment.Spec.Replicas = statusDeployment.Size
				hasClusterDeploymentChanged = true
			}
		}
		idlingCR.Status.StatusDeployments[clusterDeployment.ObjectMeta.Name] = updatedStatusDeployment
		if hasClusterDeploymentChanged {
			reconciler.updateClusterDeployment(
				context,
				clusterDeployment,
				idlingCR.Namespace,
				idleoperatorLogger)
		}
	}
}

func (reconciler *IdleOperatorReconciler) manageIdling(
	context context.Context,
	idlingCR *cachev1alpha1.IdleOperator,
	idleoperatorLogger logr.Logger) bool {
	idlingSpec := idlingCR.Spec.Idle
	for index := len(idlingSpec) - 1; index >= 0; index-- {
		depSpecs := idlingSpec[index]
		matchingDeploymentsFromLabels := &appsv1.DeploymentList{}
		err := reconciler.injectDeploymentsFromLabelAndNamespace(
			context,
			depSpecs.MatchingLabels,
			idlingCR.Namespace,
			matchingDeploymentsFromLabels,
			idleoperatorLogger)
		if err != nil {
			idleoperatorLogger.Error(err,
				"Failed to inject deployments for labels",
				depSpecs.MatchingLabels)
		}
		isStartIdling, err := cronmanagment.IsInIdleTimezone(
			depSpecs.Time,
			depSpecs.Duration,
			idleoperatorLogger.V(1))
		if err != nil {
			idleoperatorLogger.Error(
				err,
				"Failed to know if idling or not")
			continue
		}
		clusterDeployments := matchingDeploymentsFromLabels.Items
		reconciler.matchingDeploymentsChangedToDesiredState(
			context,
			isStartIdling,
			clusterDeployments,
			idlingCR,
			idleoperatorLogger)
	}
	return false
}

func (reconciler *IdleOperatorReconciler) Reconcile(
	context context.Context,
	request ctrl.Request) (ctrl.Result, error) {
	idleoperatorLogger := configureOperatorLogger(request)
	idleoperatorLogger.WithValues(
		"Operator.Name", request.Name,
		"Operator.Namespace", request.Namespace).
		Info("START RECONCILE FOR CR")
	var currentNamespaceIdlingCR cachev1alpha1.IdleOperator
	err := reconciler.Get(context, request.NamespacedName,
		&currentNamespaceIdlingCR)
	if errors.IsNotFound(err) {
		idleoperatorLogger.Info(
			"Custom Resource not found in the cluster",
			err.Error())
		return ctrl.Result{}, nil
	}
	if err != nil {
		idleoperatorLogger.Error(err,
			"Failed to List all Custom Resource")
		return ctrl.Result{}, err
	}
	if currentNamespaceIdlingCR.Status.StatusDeployments == nil {
		currentNamespaceIdlingCR.Status.StatusDeployments = make(
			map[string]cachev1alpha1.StatusDeployment)
	}
	for deploymentName, deploymentSpecs := range currentNamespaceIdlingCR.Status.StatusDeployments {
		deploymentSpecs.HasTreated = false
		currentNamespaceIdlingCR.Status.StatusDeployments[deploymentName] = deploymentSpecs
	}
	reconciler.manageIdling(
		context,
		&currentNamespaceIdlingCR,
		idleoperatorLogger)
	err = reconciler.Status().Update(context,
		&currentNamespaceIdlingCR)
	if err != nil {
		idleoperatorLogger.Error(err, "Update status failed in update")
	}
	watchCRStatus(
		currentNamespaceIdlingCR.Status.StatusDeployments,
		currentNamespaceIdlingCR.Namespace,
		idleoperatorLogger)
	reconciler.watchClusterDeployments(
		context,
		currentNamespaceIdlingCR.Namespace,
		idleoperatorLogger)
	idleoperatorLogger.WithValues(
		"Operator.Name", request.Name,
		"Operator.Namespace", request.Namespace).Info("END RECONCILE")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (reconciler *IdleOperatorReconciler) SetupWithManager(
	manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		For(&cachev1alpha1.IdleOperator{}).
		Owns(&appsv1.Deployment{}).
		Complete(reconciler)
}

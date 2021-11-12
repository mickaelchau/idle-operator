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
func watchCRStatus(statusDeployments map[string]cachev1alpha1.StatusDeployment, namespace string) {
	logrus.Infof("Deployments Status of Namespace: %s", namespace)
	for name, statusDeployment := range statusDeployments {
		logrus.Infof("Name = %s | Size = %d | Isdled = %t", name,
			statusDeployment.Size, statusDeployment.IsIdled)
	}
}

func watchListedDeployments(clusterDeployments []appsv1.Deployment, namespace string) {
	logrus.Infof("Cluster deployments of Namespace: %s", namespace)
	for index, clusclusterDeployment := range clusterDeployments {
		logrus.Infof("%d: %s", index, clusclusterDeployment.Name)
	}
}

func (reconciler *IdleOperatorReconciler) injectDeploymentsFromLabelAndNamespace(context context.Context,
	labels []map[string]string, namespace string, matchingDeployments *appsv1.DeploymentList) error {
	optionsForList := []client.ListOption{
		client.InNamespace(namespace),
	}
	for _, label := range labels {
		optionsForList = append(optionsForList, client.MatchingLabels(label))
	}
	logrus.Infof("Injecting deployments with options: %+v", optionsForList)
	err := reconciler.List(context, matchingDeployments, optionsForList...)
	if err != nil {
		logrus.Errorf("Failed to list Deployments (with namespace & label): %s", err.Error())
		return err
	}
	watchListedDeployments(matchingDeployments.Items, namespace)
	return nil
}

func (reconciler *IdleOperatorReconciler) buildCRStatus(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) error {
	haveStatusChanged := false
	statusDeployments := idlingCR.Status.StatusDeployments
	for _, clusterDeployment := range clusterDeployments {
		if !haveStatusChanged {
			haveStatusChanged = true
		}
		if statusDeployment, isInStatus := statusDeployments[clusterDeployment.ObjectMeta.Name]; isInStatus {
			logrus.Infof("already a deployment of name: %s", clusterDeployment.ObjectMeta.Name)
			if *clusterDeployment.Spec.Replicas != 0 {
				statusDeployment.Size = *clusterDeployment.Spec.Replicas
			} else {
				statusDeployment.IsIdled = true
			}
			idlingCR.Status.StatusDeployments[clusterDeployment.ObjectMeta.Name] = statusDeployment
		} else {
			logrus.Infof("No deployment of name %s", clusterDeployment.ObjectMeta.Name)
			var newStatusDeployment cachev1alpha1.StatusDeployment
			newStatusDeployment.Size = *clusterDeployment.Spec.Replicas
			newStatusDeployment.IsIdled = true
			idlingCR.Status.StatusDeployments[clusterDeployment.ObjectMeta.Name] = newStatusDeployment
		}
	}
	if haveStatusChanged {
		err := reconciler.Status().Update(context, idlingCR)
		if err != nil {
			logrus.Errorf("Update status failed in build cr: %s", err.Error())
			return err
		}
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) updateDeployment(context context.Context, idlingCR *cachev1alpha1.IdleOperator,
	clusterDeployment appsv1.Deployment, haveToDelete *bool) error {
	isClusterDeploymentChanged := false
	statusDeployments := idlingCR.Status.StatusDeployments
	if statusDeployment, isInStatus := statusDeployments[clusterDeployment.ObjectMeta.Name]; isInStatus {
		if !statusDeployment.IsIdled {
			if !*haveToDelete {
				*haveToDelete = true
			}
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
				logrus.Errorf("Update deployment: %s in namespace %s failed: %s",
					clusterDeployment.Name, idlingCR.Namespace, err.Error())
			}
		}
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) updateDeployments(context context.Context, clusterDeployments []appsv1.Deployment,
	idlingCR *cachev1alpha1.IdleOperator) error {
	haveStatusChanged := false
	for _, clusterDeployment := range clusterDeployments {
		haveToDelete := false
		reconciler.updateDeployment(context, idlingCR, clusterDeployment, &haveToDelete)
		if haveToDelete {
			delete(idlingCR.Status.StatusDeployments, clusterDeployment.ObjectMeta.Name)
			if len(idlingCR.Status.StatusDeployments) == 0 {
				idlingCR.Status.StatusDeployments = map[string]cachev1alpha1.StatusDeployment{}
			}
			if !haveStatusChanged {
				haveStatusChanged = true
			}
		}
	}
	if haveStatusChanged {
		err := reconciler.Status().Update(context, idlingCR)
		if err != nil {
			logrus.Errorf("Update status failed in update: %s", err.Error())
			return err
		}
	}
	return nil
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
			logrus.Errorf("Failed to know if idling or not: %s", err.Error())
			continue
		}
		if isStartIdling {
			matchingDeploymentsFromLabels := &appsv1.DeploymentList{}
			err := reconciler.injectDeploymentsFromLabelAndNamespace(context, depSpecs.MatchingLabels, idlingCR.Namespace,
				matchingDeploymentsFromLabels)
			if err != nil {
				logrus.Errorf("Failed to inject deployments for labels %s and namespace %s: %s", depSpecs.MatchingLabels,
					idlingCR.Namespace, err.Error())
			}
			allMatchingDeployments.Items = append(allMatchingDeployments.Items, matchingDeploymentsFromLabels.Items...)
		} else {
			logrus.Infof("Deployments for label %s and namespace %s does not match timezone", depSpecs.MatchingLabels,
				idlingCR.Namespace)
		}
	}
	//all matching deploys => all deploys I have to iddle
	reconciler.buildCRStatus(context, allMatchingDeployments.Items, &idlingCR)
	allClusterDeployments := &appsv1.DeploymentList{}
	err := reconciler.injectDeploymentsFromLabelAndNamespace(context, nil, idlingCR.Namespace, allClusterDeployments)
	if err != nil {
		logrus.Errorf("Failed to inject deployments: %s", err.Error())
		return err
	}
	//return nil
	err = reconciler.updateDeployments(context, allClusterDeployments.Items, &idlingCR)
	if err != nil {
		logrus.Errorf("Failed to update cluster deployments: %s", err.Error())
		return err
	}
	return nil

}

func (reconciler *IdleOperatorReconciler) Reconcile(context context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = oplog.FromContext(context)
	var allNamespaceIdlingCR cachev1alpha1.IdleOperatorList
	err := reconciler.List(context, &allNamespaceIdlingCR)
	if errors.IsNotFound(err) {
		logrus.Infof("Custom Resource not found in the cluster at the namespace %s: %s", request.Namespace, err.Error())
		return ctrl.Result{}, nil
	}
	if err != nil {
		logrus.Errorf("Failed to List all Custom Resource: %s", err.Error())
		return ctrl.Result{}, err
	}
	for _, idlingCR := range allNamespaceIdlingCR.Items {
		if idlingCR.Status.StatusDeployments == nil {
			idlingCR.Status.StatusDeployments = make(map[string]cachev1alpha1.StatusDeployment)
		}
		err = reconciler.manageIdling(context, idlingCR)
		if err != nil {
			logrus.Errorf("Something goes wrong while idling: %s", err.Error())
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

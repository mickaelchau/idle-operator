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

func watchListedDeployments(clusterDeployments []appsv1.Deployment, namespace string, optionsForList []client.ListOption) {
	logrus.Infof("Cluster deployments with options: %+v", optionsForList)
	for index, clusterDeployment := range clusterDeployments {
		logrus.Infof("Deployment: %d | Name: %s", index, clusterDeployment.Name)
	}
}

func (reconciler *IdleOperatorReconciler) watchClusterDeployments(context context.Context, namespace string) {
	logrus.Infof("Cluster deployments of Namespace: %s", namespace)
	var allNamespaceDeployments appsv1.DeploymentList
	err := reconciler.List(context, &allNamespaceDeployments, client.InNamespace(namespace))
	if err != nil {
		logrus.Printf("Failed to list all deployments of %s: %s", namespace, err.Error())
	}
	for index, clusterDeployment := range allNamespaceDeployments.Items {
		logrus.Infof("Deployment: %d | Name: %s | Replicas: %d", index, clusterDeployment.Name,
			*clusterDeployment.Spec.Replicas)
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
	err := reconciler.List(context, matchingDeployments, optionsForList...)
	if err != nil {
		logrus.Errorf("Failed to list Deployments (with namespace & label): %s", err.Error())
		return err
	}
	watchListedDeployments(matchingDeployments.Items, namespace, optionsForList)
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
		statusDeployment, isInStatus := statusDeployments[clusterDeployment.ObjectMeta.Name]
		if isInStatus {
			logrus.Infof("There's already a deployment of name: %s in Status", clusterDeployment.ObjectMeta.Name)
			if *clusterDeployment.Spec.Replicas != 0 {
				statusDeployment.Size = *clusterDeployment.Spec.Replicas
			} else {
				statusDeployment.IsIdled = true
			}
			idlingCR.Status.StatusDeployments[clusterDeployment.ObjectMeta.Name] = statusDeployment
		} else {
			logrus.Infof("There's no deployment of name %s in Status", clusterDeployment.ObjectMeta.Name)
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
	hasClusterDeploymentChanged := false
	statusDeployments := idlingCR.Status.StatusDeployments
	statusDeployment, isInStatus := statusDeployments[clusterDeployment.ObjectMeta.Name]
	if isInStatus {
		if !statusDeployment.IsIdled {
			if !*haveToDelete {
				*haveToDelete = true
			}
			clusterDeployment.Spec.Replicas = &statusDeployment.Size
			hasClusterDeploymentChanged = true
		}
		if statusDeployment.IsIdled && *clusterDeployment.Spec.Replicas != 0 {
			*clusterDeployment.Spec.Replicas = 0
			hasClusterDeploymentChanged = true
		}
		if hasClusterDeploymentChanged {
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
		matchingDeploymentsFromLabels := &appsv1.DeploymentList{}
		err := reconciler.injectDeploymentsFromLabelAndNamespace(context, depSpecs.MatchingLabels, idlingCR.Namespace,
			matchingDeploymentsFromLabels)
		if err != nil {
			logrus.Errorf("Failed to inject deployments for labels %s and namespace %s: %s", depSpecs.MatchingLabels,
				idlingCR.Namespace, err.Error())
		}
		isStartIdling, err := cronmanagment.IsInIdleTimezone(depSpecs.Time, depSpecs.Duration)
		if err != nil {
			logrus.Errorf("Failed to know if idling or not: %s", err.Error())
			continue
		}
		if isStartIdling {
			logrus.Infof("Deployments match timezone")
			allMatchingDeployments.Items = append(allMatchingDeployments.Items, matchingDeploymentsFromLabels.Items...)
			/*
				err = reconciler.buildCRStatus(context, matchingDeploymentsFromLabels.Items, &idlingCR)
				if err != nil {
					logrus.Errorf("Fail to build CR Status: %s", err.Error())
				}
			*/
		} else {
			logrus.Infof("Deployments does not match timezone")
		}
		/*
			err = reconciler.updateDeployments(context, matchingDeploymentsFromLabels.Items, &idlingCR)
			if err != nil {
				logrus.Errorf("Failed to update cluster deployments: %s", err.Error())
				return err
			}
		*/
	}
	err := reconciler.buildCRStatus(context, allMatchingDeployments.Items, &idlingCR)
	if err != nil {
		logrus.Errorf("Fail to build CR Status: %s", err.Error())
	}
	err = reconciler.updateDeployments(context, allMatchingDeployments.Items, &idlingCR)
	if err != nil {
		logrus.Errorf("Failed to update cluster deployments: %s", err.Error())
		return err
	}
	return nil
}

func (reconciler *IdleOperatorReconciler) Reconcile(context context.Context, request ctrl.Request) (ctrl.Result, error) {
	_ = oplog.FromContext(context)
	logrus.Infof("START RECONCILE FOR CR: %s IN NAMESPACE: %s", request.Name, request.Namespace)
	var currentNamespaceIdlingCR cachev1alpha1.IdleOperator
	err := reconciler.Get(context, request.NamespacedName, &currentNamespaceIdlingCR)
	if errors.IsNotFound(err) {
		logrus.Infof("Custom Resource not found in the cluster at the namespace %s: %s", request.Namespace, err.Error())
		return ctrl.Result{}, nil
	}
	if err != nil {
		logrus.Errorf("Failed to List all Custom Resource: %s", err.Error())
		return ctrl.Result{}, err
	}
	if currentNamespaceIdlingCR.Status.StatusDeployments == nil {
		currentNamespaceIdlingCR.Status.StatusDeployments = make(map[string]cachev1alpha1.StatusDeployment)
	}
	err = reconciler.manageIdling(context, currentNamespaceIdlingCR)
	if err != nil {
		logrus.Errorf("Something goes wrong while idling: %s", err.Error())
		return ctrl.Result{}, err
	}
	watchCRStatus(currentNamespaceIdlingCR.Status.StatusDeployments, currentNamespaceIdlingCR.Namespace)
	reconciler.watchClusterDeployments(context, currentNamespaceIdlingCR.Namespace)
	logrus.Infof("END RECONCILE FOR CR: %s IN NAMESPACE: %s", request.Name, request.Namespace)
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

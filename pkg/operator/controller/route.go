/*
Copyright 2018 The CDI Authors.

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
	"fmt"

	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	uploadProxyServiceName = "cdi-uploadproxy"
	uploadProxyRouteName   = uploadProxyServiceName
	uploadProxyCABundle    = "cdi-uploadproxy-signer-bundle"
)

func ensureUploadProxyRouteExists(ctx context.Context, logger logr.Logger, c client.Client, scheme *runtime.Scheme, owner metav1.Object) error {
	namespace := owner.GetNamespace()
	if namespace == "" {
		return fmt.Errorf("cluster scoped owner not supported")
	}

	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{Namespace: namespace, Name: uploadProxyCABundle}
	if err := c.Get(ctx, key, cm); err != nil {
		if errors.IsNotFound(err) {
			logger.V(3).Info("upload proxy ca cert doesn't exist yet")
			return nil
		}
		return err
	}

	cert, exists := cm.Data["ca-bundle.crt"]
	if !exists {
		return fmt.Errorf("unexpected ConfigMap format, 'ca-bundle.crt' key missing")
	}

	cr, err := cc.GetActiveCDI(ctx, c)
	if err != nil {
		return err
	}
	if cr == nil {
		return fmt.Errorf("no active CDI")
	}
	installerLabels := util.GetRecommendedInstallerLabelsFromCr(cr)

	desiredRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uploadProxyRouteName,
			Namespace: namespace,
			Labels: map[string]string{
				"cdi.kubevirt.io": "",
			},
			Annotations: map[string]string{
				// long timeout here to make sure client conection doesn't die during qcow->raw conversion
				"haproxy.router.openshift.io/timeout": "60m",
			},
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: uploadProxyServiceName,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationReencrypt,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				DestinationCACertificate:      string(cert),
			},
		},
	}
	util.SetRecommendedLabels(desiredRoute, installerLabels, "cdi-operator")

	currentRoute := &routev1.Route{}
	key = client.ObjectKey{Namespace: namespace, Name: uploadProxyRouteName}
	err = c.Get(ctx, key, currentRoute)
	if err == nil {
		if currentRoute.Spec.To.Kind != desiredRoute.Spec.To.Kind ||
			currentRoute.Spec.To.Name != desiredRoute.Spec.To.Name ||
			currentRoute.Spec.TLS == nil ||
			currentRoute.Spec.TLS.Termination != desiredRoute.Spec.TLS.Termination ||
			currentRoute.Spec.TLS.DestinationCACertificate != desiredRoute.Spec.TLS.DestinationCACertificate {
			currentRoute.Spec = desiredRoute.Spec
			return c.Update(ctx, currentRoute)
		}

		return nil
	}

	if meta.IsNoMatchError(err) {
		// not in openshift
		logger.V(3).Info("No match error for Route, must not be in openshift")
		return nil
	}

	if !errors.IsNotFound(err) {
		return err
	}

	if err = controllerutil.SetControllerReference(owner, desiredRoute, scheme); err != nil {
		return err
	}

	return c.Create(ctx, desiredRoute)
}

func (r *ReconcileCDI) watchRoutes() error {
	err := r.uncachedClient.List(context.TODO(), &routev1.RouteList{}, &client.ListOptions{
		Namespace: util.GetNamespace(),
		Limit:     1,
	})
	if err == nil {
		return r.controller.Watch(source.Kind(r.getCache(), &routev1.Route{}), enqueueCDI(r.client))
	}
	if meta.IsNoMatchError(err) {
		log.Info("Not watching Routes")
		return nil
	}

	return err
}

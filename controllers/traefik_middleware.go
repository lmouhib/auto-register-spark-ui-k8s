package controllers

import (
	"context"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	logger "k8s.io/klog/v2"
)

func ManageTraefikMiddleware(dynamicClient dynamic.Interface, namespace, action string, authenticationSecret *string) error {

	// Define the GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "middlewares",
	}

	switch action {
	case "create":

		spec := map[string]interface{}{
			"stripPrefixRegex": map[string]interface{}{
				"regex": []interface{}{
					"^/[^/]+(/|$)",
				},
			},
		}

		middlewareName := "spark-ui-url-strip"

		createMiddlewareObject(dynamicClient, gvr, middlewareName, namespace, spec)

		if authenticationSecret != nil {

			middlewareName = "spark-ui-url-auth"

			spec = map[string]interface{}{
				"basicAuth": map[string]interface{}{
					"secret": authenticationSecret,
				},
			}

			createMiddlewareObject(dynamicClient, gvr, middlewareName, namespace, spec)
		}

	case "delete":
		// Delete the Middleware object
		err := dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), "spark-ui-url-strip", metav1.DeleteOptions{})

		if authenticationSecret != nil {
			err = dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), "spark-ui-url-auth", metav1.DeleteOptions{})
		}
		if err != nil {
			logger.Errorf("error deleting Middleware: %v", err)
			return err
		}

		logger.Infof("Middleware deleted successfully")

	default:
		logger.Errorf("invalid action: %v", action)
		return errors.New("invalid action")
	}

	return nil
}

func createMiddlewareObject(
	dynamicClient dynamic.Interface,
	gvr schema.GroupVersionResource,
	middlewareName string,
	namespace string,
	spec map[string]interface{}) {

	middleware := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "Middleware",
			"metadata": map[string]interface{}{
				"name":      middlewareName,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}

	// Create the Middleware object
	_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), middleware, metav1.CreateOptions{})
	if err != nil {
		logger.Errorf("error creating Middleware: %v", err)
	}
}

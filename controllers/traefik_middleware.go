package controllers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	logger "k8s.io/klog/v2"
)

func ManageTraefikMiddleware(namespace, action string, authenticationSecret *string) error {

	// Extract the config from the clientset
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Errorf("error creating in-cluster config: %v", err)
		return err
	}

	// Create the dynamic client to create generic resources
	// This is needed since Treafik Middleware is a custom resource
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Errorf("error creating dynamic client: %v", err)
		return err
	}

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
		err = dynamicClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), "spark-ui-url-strip", metav1.DeleteOptions{})

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
		return err
	}

	return nil
}

func createMiddlewareObject(
	dynamicClient *dynamic.DynamicClient,
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

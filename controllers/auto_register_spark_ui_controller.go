package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	logger "k8s.io/klog/v2"
)

// Function to create or update the Ingress object
// It takes the ingress path, either creates a new ingress object or patch the existing one
func createOrUpdateSparkUIIngressObject(
	ctx context.Context,
	clientset kubernetes.Interface,
	service *v1.Service,
	ingressPath networkingv1.HTTPIngressPath,
	ingressName string,
	ingressType string,
	authenticationSecret string) {

	ingressClient := clientset.NetworkingV1().Ingresses(service.Namespace)

	ingress, err := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {

			var ingressClassName *string

			if ingressType == "traefik" {
				// Create the Traefik middleware
				err := ManageTraefikMiddleware(service.Namespace, "create", &authenticationSecret)
				if err != nil {
					logger.Error(err)
					return
				}
				ingressClassName = nil
			} else {
				ingressClassName = func() *string { s := "nginx"; return &s }()
			}

			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        ingressName,
					Namespace:   service.Namespace,
					Annotations: createIngressAnnotations(ingressType, service, &authenticationSecret),
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ingressClassName,
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{ingressPath},
								},
							},
						},
					},
				},
			}

			_, err := clientset.NetworkingV1().Ingresses(service.Namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})

			logger.Infof("Created Ingress %v", ingressName)
			if err != nil {
				logger.Error(err)
			}

		} else {
			logger.Error(err, "Error getting Ingress")
		}

	} else {

		logger.Infof("Ingress %v already exists", ingressName)
		logger.Infof("Updating ingress with %v", ingressPath)

		ingressCopy := ingress.DeepCopy()
		ingressCopy.Spec.Rules[0].HTTP.Paths = append(ingress.Spec.Rules[0].HTTP.Paths,
			ingressPath,
		)

		// Convert the updated rules to JSON
		patchData, err := json.Marshal(map[string]interface{}{
			"spec": map[string]interface{}{
				"rules": ingressCopy.Spec.Rules,
			},
		})
		if err != nil {
			logger.Error(err)
			return
		}

		// Patch the ingress with the updated rules
		result, err := clientset.NetworkingV1().Ingresses(service.Namespace).Patch(
			ctx,
			ingressCopy.Name,
			types.StrategicMergePatchType,
			patchData,
			metav1.PatchOptions{
				FieldValidation: "Strict",
			},
		)

		logger.V(4).Infof("The updated Ingress %v", result)
		if err != nil {
			logger.Error(err)
		}
	}
}

// Add function called by the informer when a service is created
// It takes the service object, creates the ingress path
// and calls the function responsible for creating or patching the Ingress object
func Add(
	ctx context.Context,
	clientset kubernetes.Interface,
	service *v1.Service,
	namespacedIngressPath bool,
	ingressName string,
	ingressType string,
	authenticationSecret *string) {

	logger.Infof("Create ingress rule for Spark Application : %s \n", service.GetName())

	//get the value of the service selector called "spark-app-name"
	//this value is used to route the traffic to the right Spark UI
	sparkAppName := service.Spec.Selector["spark-app-name"]

	logger.Infof("Spark App Name: %v", sparkAppName)

	var sparkUIPath string = buildSparkUIPath(namespacedIngressPath, service, sparkAppName)

	logger.Infof("Spark UI path: %v", sparkUIPath)

	ingressPath := networkingv1.HTTPIngressPath{
		PathType: func() *networkingv1.PathType { p := networkingv1.PathTypeImplementationSpecific; return &p }(),
		Path:     sparkUIPath,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: service.GetName(),
				Port: networkingv1.ServiceBackendPort{
					Number: 4040,
				},
			},
		},
	}

	//Call the function responsible for creating or patching the Ingress object
	createOrUpdateSparkUIIngressObject(ctx, clientset, service, ingressPath, ingressName, ingressType, *authenticationSecret)

}

// Delete function called by the informer when a service is deleted
// It takes the servicename, either deletes the ingress object or patch the existing one by
// removing the path that matches the sparkAppName
func Delete(
	ctx context.Context,
	clientset *kubernetes.Clientset,
	service *v1.Service,
	namespacedIngressPath bool,
	ingressName string,
	ingressType string,
	authenticationSecret *string) {

	var sparkAppName string

	namespace := service.Namespace

	// Get the existing ingress
	ingress, err := clientset.NetworkingV1().Ingresses(namespace).Get(ctx, ingressName, metav1.GetOptions{})
	if err != nil {
		logger.Errorf("failed to get ingress: %v", err)
		return
	}

	sparkAppName = service.Spec.Selector["spark-app-name"]

	var sparkUIPath string = buildSparkUIPath(namespacedIngressPath, service, sparkAppName)

	logger.Infof("Spark UI path to remove: %v", sparkUIPath)

	// Deep copy the existing ingress
	ingressCopy := ingress.DeepCopy()

	// Find and remove the path that matches the sparkAppName
	updatedPaths := []networkingv1.HTTPIngressPath{}
	for _, path := range ingressCopy.Spec.Rules[0].HTTP.Paths {
		if path.Path != sparkUIPath {
			updatedPaths = append(updatedPaths, path)
		}
	}

	// Check if there are no paths left
	if len(updatedPaths) == 0 {
		// Delete the ingress
		err = clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, ingressCopy.Name, metav1.DeleteOptions{})
		if err != nil {
			logger.Errorf("failed to delete ingress: %v", err)
			return
		}
		log.Printf("Deleted ingress %s as it had no paths left", ingressName)

		// Delete the Traefik middleware
		if ingressType == "traefik" {
			ManageTraefikMiddleware(namespace, "delete", authenticationSecret)
			log.Printf("Deleted middleware for authentication and url strip as ingress object is deleted")
		}
		return
	}

	// Update the ingress paths
	ingressCopy.Spec.Rules[0].HTTP.Paths = updatedPaths

	// Convert the updated rules to JSON
	patchData, err := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{
			"rules": ingressCopy.Spec.Rules,
		},
	})
	if err != nil {
		logger.Errorf("failed to marshal patch data: %v", err)
		return
	}

	// Patch the ingress with the updated rules
	_, err = clientset.NetworkingV1().Ingresses(namespace).Patch(
		ctx,
		ingressCopy.Name,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{
			FieldValidation: "Strict",
		},
	)
	if err != nil {
		logger.Errorf("failed to patch ingress: %v", err)
		return
	}

	log.Printf("Deleted path for sparkAppName %s from ingress %s", sparkAppName, ingressName)
}

func buildSparkUIPath(namespacedIngressPath bool, service *v1.Service, sparkAppName string) string {
	var sparkUIPath string

	if namespacedIngressPath {
		sparkUIPath = "/" + service.Namespace + "/" + sparkAppName + "(/|$)(.*)"
	} else {
		sparkUIPath = "/" + sparkAppName + "(/|$)(.*)"
	}

	return sparkUIPath
}
func createIngressAnnotations(
	ingressType string,
	service *v1.Service,
	authenticationSecret *string) map[string]string {

	switch ingressType {

	case "nginx":

		annotationObject := map[string]string{
			"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
			"nginx.ingress.kubernetes.io/use-regex":      "true",
		}

		if authenticationSecret != nil {
			annotationObject["nginx.ingress.kubernetes.io/auth-type"] = "basic"
			annotationObject["nginx.ingress.kubernetes.io/auth-secret"] = *authenticationSecret
			annotationObject["nginx.ingress.kubernetes.io/auth-realm"] = "Authentication Required"
		}

		return annotationObject

	case "traefik":

		var middlewareValue string

		if authenticationSecret != nil {
			middlewareValueList := []string{
				fmt.Sprintf("%s-%s@kubernetescrd", service.Namespace, "spark-ui-url-auth"),
				fmt.Sprintf("%s-%s@kubernetescrd", service.Namespace, "spark-ui-url-strip"),
			}
			middlewareValue = strings.Join(middlewareValueList, ",\n")
		} else {
			middlewareValue = fmt.Sprintf("%s-%s@kubernetescrd", service.Namespace, "spark-ui-url-strip")
		}

		annotationObject := map[string]string{
			"traefik.ingress.kubernetes.io/router.middlewares": middlewareValue,
			"traefik.ingress.kubernetes.io/router.entrypoints": "web",
			"traefik.ingress.kubernetes.io/router.pathmatcher": "PathRegexp",
		}

		return annotationObject

	default:
		logger.Errorf("unsupported ingress type: %s", ingressType)
		return nil
	}

}

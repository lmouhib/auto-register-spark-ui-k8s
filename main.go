package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lmouhib/auto-register-k8s-spark-ui/controllers"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	logger "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {

	ctx := signals.SetupSignalHandler()

	config, err := rest.InClusterConfig()

	if err != nil {
		logger.Fatal(err, "Error building kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal(err, "Error building clientset")
	}
	logger.Infof("Connected to kubernetes cluster")

	// Check for the environment variable for spark service selector
	labelKey := os.Getenv("SPARK_LABEL_SERVICE_SELECTOR")
	if labelKey != "" {
		logger.Infof("Using environment variable for spark service selector: %v", labelKey)
	} else {
		labelKey = "spark-app-selector"
		logger.Infof("Using default value for spark service selector: %v", labelKey)
	}

	// Check for the environment variable for ingress name
	ingressName := os.Getenv("INGRESS_NAME")
	if ingressName != "" {
		logger.Infof("The ingress will be called: %v", ingressName)
	} else {
		ingressName = "auto-register-spark-ui-ingress"
		logger.Infof("Using default ingress name: %v", labelKey)
	}

	// Check for the environment variable for ingress type
	ingressType := os.Getenv("INGRESS_TYPE")
	if ingressType != "" {
		logger.Infof("The ingress type is: %v", ingressType)
	} else {
		ingressType = "traefik"
		logger.Infof("Using default ingress type: %v", ingressType)
	}

	//create the informer factory
	//if namespace is not empty, create a filtered informer factory
	//We will only liste to service created in the provided namespace
	var informerFactory informers.SharedInformerFactory

	sparkNamespace := os.Getenv("SPARK_NAMESPACE")
	if sparkNamespace != "" {
		logger.Infof("Namespace set, listening to Spark created service in namespace: %v", sparkNamespace)

		informerFactory = informers.NewFilteredSharedInformerFactory(
			clientset,
			time.Second*30,
			sparkNamespace,
			nil)

	} else {
		logger.Infof("No Spark namespace set, listening for Spark created service across all namespaces")
		informerFactory = informers.NewSharedInformerFactory(clientset, time.Second*30)
	}

	//Check if there is a need to add the namespace to the path
	//This is useful if a job with the same name is created in different namespaces
	namespacedIngressPathEnv := os.Getenv("NAMESPACED_INGRESS_PATH")

	if namespacedIngressPathEnv == "" {
		logger.Infof("No environment variable set for NAMESPACED_INGRESS_PATH, using default value")
		namespacedIngressPathEnv = "false"
	}

	namespacedIngressPath, err := strconv.ParseBool(namespacedIngressPathEnv)

	if err != nil {
		logger.Fatal(err, "Error parsing NAMESPACED_INGRESS_PATH")
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	// Function to check if a service has a specific label
	hasLabel := func(service *v1.Service, labelKey string) bool {
		labels := service.GetLabels()
		_, ok := labels[labelKey]
		return ok
	}

	// Create an informer to watch for changes in services
	informer := informerFactory.Core().V1().Services().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*v1.Service)
			if hasLabel(service, labelKey) {
				logger.Infof("Service %v created with label %v\n", service.GetName(), labelKey)
				controllers.Add(ctx, clientset, service, namespacedIngressPath, ingressName, ingressType)
			}
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*v1.Service)
			if hasLabel(service, labelKey) {
				logger.Infof("Service %v deleted with label %v \n", service.GetName(), labelKey)
				controllers.Delete(ctx, clientset, service, namespacedIngressPath, ingressName, ingressType)
			}
		},
	})
	go informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		logger.Fatalf("Error syncing cache")
	}

	wait.Until(func() {
		fmt.Println("Listening for changes in services")
	}, time.Second*10, stopCh)
}

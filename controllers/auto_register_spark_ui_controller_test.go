package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// Mock logger to capture log output
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Error(err error) {
	m.Called(err)
}

func TestAdd(t *testing.T) {
	// Create a mock logger
	mockLogger := new(MockLogger)

	// Create a fake Kubernetes clientset
	var clientset kubernetes.Interface = fake.NewSimpleClientset()
	var dynamicClient dynamic.Interface = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	// Define other parameters
	ctx := context.TODO()
	namespacedIngressPath := false
	ingressName := "test-ingress"
	ingressType := "nginx"
	authenticationSecret := new(string)

	// Create a mock service with a specific selector
	defaultService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"spark-app-name": "test-spark-app",
			},
		},
	}

	// Expect the logger to receive the correct log message
	mockLogger.On("Infof", "Create ingress rule for Spark Application : %s \n", defaultService.GetName()).Return()

	// Call the Add function
	Add(ctx, clientset, dynamicClient, defaultService, namespacedIngressPath, ingressName, ingressType, authenticationSecret)

	// Verify that the correct ingress was created
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)

	ingress := ingresses.Items[0]
	assert.Equal(t, ingressName, ingress.Name)
	assert.Equal(t, defaultService.Namespace, "default")
	assert.Equal(t, "/test-spark-app(/|$)(.*)", ingress.Spec.Rules[0].HTTP.Paths[0].Path)

	namespacedIngressPath = true

	// Create a mock service with a specific selector
	defaultService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"spark-app-name": "default-spark-app",
			},
		},
	}

	// Call the Add function
	Add(ctx, clientset, dynamicClient, defaultService, namespacedIngressPath, ingressName, ingressType, authenticationSecret)

	// Verify that the correct ingress was created
	ingresses, err = clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, ingresses.Items, 1)

	ingress = ingresses.Items[0]
	assert.Equal(t, defaultService.Namespace, "default")
	assert.Equal(t, "/default/default-spark-app(/|$)(.*)", ingress.Spec.Rules[0].HTTP.Paths[1].Path)

	// ingressName = "traefik-ingress"
	// ingressType = "traefik"
	// namespacedIngressPath = false

	// // Create a mock service with a specific selector
	// defaultService = &v1.Service{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Namespace: "default",
	// 	},
	// 	Spec: v1.ServiceSpec{
	// 		Selector: map[string]string{
	// 			"spark-app-name": "default-spark-app",
	// 		},
	// 	},
	// }

	// Add(ctx, clientset, defaultService, namespacedIngressPath, ingressName, ingressType, authenticationSecret)

	// ingresses, _ = clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	// ingress = ingresses.Items[1]

	// assert.Equal(t, "default-spark-ui-url-strip@kubernetescrd", ingress.Annotations["traefik.ingress.kubernetes.io/router.middlewares"], "Ingress does not have the expected Traefik middleware annotation")

}

package controllers

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	//Set environment variables used to configure the controller behavior
	os.Setenv("NAMESPACED_INGRESS_PATH", "false")
	os.Setenv("SPARK_NAMESPACE", "default")
	os.Setenv("AUTHENTICATION_SETUP", "auth-secret")

	// Create a fake Kubernetes clientset
	var clientset kubernetes.Interface = fake.NewSimpleClientset()

	// Define other parameters
	ctx := context.TODO()
	namespacedIngressPath := true
	ingressName := "test-ingress"
	ingressType := "test-type"
	authenticationSecret := new(string)

	// Create a mock service with a specific selector
	serviceCustomLabel := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"custom-spark-app-selector": "my-label",
				"spark-app-name":            "test-spark-app",
			},
		},
	}

	os.Setenv("SPARK_LABEL_SERVICE_SELECTOR", "custom-spark-app-selector")

	// Expect the logger to receive the correct log message
	mockLogger.On("Infof", "Create ingress rule for Spark Application : %s \n", serviceCustomLabel.GetName()).Return()

	// Call the Add function
	Add(ctx, clientset, serviceCustomLabel, namespacedIngressPath, ingressName, ingressType, authenticationSecret)

	// Verify that the service selector was accessed correctly
	assert.Equal(t, "my-label", serviceCustomLabel.Spec.Selector["custom-spark-app-selector"])

	// Verify that the correct ingress was created
	ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, ingresses.Items, 1)

	ingress := ingresses.Items[0]
	assert.Equal(t, ingressName, ingress.Name)
	assert.Equal(t, serviceCustomLabel.Namespace, "default")
	assert.Equal(t, "/default/test-spark-app(/|$)(.*)", ingress.Spec.Rules[0].HTTP.Paths[0].Path)

}

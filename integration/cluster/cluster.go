package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mt-sre/devkube/dev"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	pkoapis "package-operator.run/apis"
	hypershiftv1beta1 "package-operator.run/package-operator/internal/controllers/hostedclusters/hypershift/v1beta1"

	"sigs.k8s.io/kind/pkg/cluster"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
)

const (
	defaultWaitTimeout  = 20 * time.Second
	defaultWaitInterval = 1 * time.Second
)

type testEnv struct {
	// Client pointing to the e2e test cluster.
	Client client.Client
	// Scheme used by created clients.
	Scheme *runtime.Scheme

	Waiter *dev.Waiter

	// PackageOperatorNamespace is the namespace that the Package Operator is running in.
	// Needs to be auto-discovered, because OpenShift CI is installing the Operator in a non deterministic namespace.
	PackageOperatorNamespace string

	TestStubImage string
	// SuccessTestPackageImage points to an image to use to test Package installation.
	SuccessTestPackageImage string
	FailureTestPackageImage string

	KubeConfig string
}

func clientConfigFromCluster(t *testing.T) string {
	dev.NewCluster(".", dev.WithNewRestConfigFunc())
	t.Helper()

	provider := cluster.NewProvider()
	name := t.Name()

	name = strings.ToLower(name)

	err := provider.Create(name)
	if err != nil {
		panic(err)
	}

	t.Cleanup(func() {
		if err := provider.Delete(name, ""); err != nil {
			panic(err)
		}
	})

	cfg, err := provider.KubeConfig(name, false)
	if err != nil {
		panic(err)
	}

	return cfg
}

func New(ctx context.Context, t *testing.T) *testEnv {
	t.Helper()

	schemeBuilder := runtime.SchemeBuilder{clientgoscheme.AddToScheme, pkoapis.AddToScheme, hypershiftv1beta1.AddToScheme}
	scheme := runtime.NewScheme()

	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		panic(fmt.Errorf("could not load schemes: %w", err))
	}

	configStr := clientConfigFromCluster(t)

	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(configStr))
	if err != nil {
		panic(err)
	}

	client, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		panic(fmt.Errorf("creating runtime testEnv.Client: %w", err))
	}

	successTestPackage, ok := os.LookupEnv("PKO_TEST_SUCCESS_PACKAGE_IMAGE")
	if !ok {
		panic("PKO_TEST_SUCCESS_PACKAGE_IMAGE not set!")
	}

	testStubImage, ok := os.LookupEnv("PKO_TEST_STUB_IMAGE")
	if !ok {
		panic("PKO_TEST_STUB_IMAGE not set!")
	}

	waiter := dev.NewWaiter(client, scheme, dev.WithTimeout(defaultWaitTimeout), dev.WithInterval(defaultWaitInterval))

	testEnv := &testEnv{
		client,
		scheme,
		waiter,
		"",
		testStubImage,
		successTestPackage,
		"localhost/does-not-exist",
		configStr,
	}

	bootstrap(ctx, testEnv)

	testEnv.PackageOperatorNamespace = findPackageOperatorNamespace(ctx, client)

	return testEnv
}

func bootstrap(ctx context.Context, testEnv *testEnv) {
	filePath := filepath.Join("..", "..", "config", "self-bootstrap-job.yaml")

	fileYaml, err := os.ReadFile(filePath)
	if err != nil {
		panic(fmt.Errorf("reading %s: %w", filePath, err))
	}

	// Trim empty starting and ending objects
	fileYaml = bytes.Trim(fileYaml, "-\n")

	// Split for every included yaml document.
	for i, yamlDocument := range bytes.Split(fileYaml, []byte("---\n")) {
		obj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(yamlDocument, &obj); err != nil {
			panic(fmt.Errorf("unmarshalling yaml document at index %d: %w", i, err))
		}

		err := testEnv.Client.Create(ctx, &obj)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			panic(fmt.Sprintf("xxxxx%vxxxx", string(yamlDocument)))
			panic(fmt.Errorf("creating object: %w", err))
		}

		if err := testEnv.Waiter.WaitForReadiness(ctx, &obj); err != nil {
			var unknownTypeErr *dev.UnknownTypeError
			switch {
			case err == nil:
			case errors.As(err, &unknownTypeErr):
			// A lot of types don't require waiting for readiness,
			// so we should not error in cases when object types
			// are not registered for the generic wait method.
			default:
				panic(fmt.Errorf("waiting for object: %w", err))
			}
		}
	}

	// Bootstrap job is cleaning itself up after completion, so we can't wait for Condition Completed=True.
	// See self-bootstrap-job .spec.ttlSecondsAfterFinished: 0
	err = testEnv.Waiter.WaitToBeGone(
		ctx,
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "package-operator-bootstrap",
				Namespace: "package-operator-system",
			},
		},
		func(obj client.Object) (done bool, err error) { return },
		dev.WithTimeout(10*time.Minute),
	)
	if err != nil {
		panic(err)
	}

	err = testEnv.Waiter.WaitForCondition(ctx, &corev1alpha1.ClusterPackage{
		ObjectMeta: metav1.ObjectMeta{Name: "package-operator"},
	}, corev1alpha1.PackageAvailable, metav1.ConditionTrue)
	if err != nil {
		panic(err)
	}

	// Create a new secret for the kubeconfig
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service-network-admin-kubeconfig",
			Namespace: "default",
		},
		Data: map[string][]byte{"kubeconfig": []byte(testEnv.KubeConfig)},
	}

	// Deploy the secret with the new kubeconfig
	_ = testEnv.Client.Delete(ctx, secret)
	if err := testEnv.Client.Create(ctx, secret); err != nil {
		panic(fmt.Errorf("deploy kubeconfig secret: %w", err))
	}
}

func findPackageOperatorNamespace(ctx context.Context, client client.Client) (packageOperatorNamespace string) {
	// discover packageOperator Namespace
	deploymentList := &appsv1.DeploymentList{}
	// We can't use a label-selector, because OLM is overriding the deployment labels...
	if err := client.List(ctx, deploymentList); err != nil {
		panic(fmt.Errorf("listing package-operator deployments on the cluster: %w", err))
	}
	var packageOperatorDeployments []appsv1.Deployment
	for _, deployment := range deploymentList.Items {
		if deployment.Name == "package-operator-manager" {
			packageOperatorDeployments = append(packageOperatorDeployments, deployment)
		}
	}
	switch len(packageOperatorDeployments) {
	case 0:
		panic("no packageOperator deployment found on the cluster")
	case 1:
		packageOperatorNamespace = packageOperatorDeployments[0].Namespace
	default:
		panic("multiple packageOperator deployments found on the cluster")
	}
	return
}

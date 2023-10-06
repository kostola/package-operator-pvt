package testutil

import (
	pkocore "package-operator.run/apis/core/v1alpha1"
	pkomanifest "package-operator.run/apis/manifests/v1alpha1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	hypershift "package-operator.run/internal/controllers/hostedclusters/hypershift/v1beta1"
)

var Scheme = runtime.NewScheme()

func init() {
	if err := pkomanifest.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := pkocore.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := apps.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := apiextensions.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := core.AddToScheme(Scheme); err != nil {
		panic(err)
	}
	if err := hypershift.AddToScheme(Scheme); err != nil {
		panic(err)
	}
}

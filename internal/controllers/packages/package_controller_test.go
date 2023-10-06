package packages_test

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	manifestsv1alpha1 "package-operator.run/apis/manifests/v1alpha1"
	"package-operator.run/internal/controllers/packages"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/testutil"
)

func TestReconcile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	log := testr.New(t)
	client := testutil.NewClient()
	ipm := &imagePullerMock{}

	ipm.On("Pull", mock.Anything, mock.Anything).Once().Return(packagecontent.Files{}, nil)

	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {}).Return(nil)
	client.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {}).Return(nil)
	client.StatusMock.On("Update", mock.Anything, mock.Anything, mock.Anything).Once().Run(func(args mock.Arguments) {}).Return(nil)

	controller := packages.NewPackageController(client, log, testutil.Scheme, ipm, nil, nil)
	controller.SetEnvironment(&manifestsv1alpha1.PackageEnvironment{})

	name := types.NamespacedName{Namespace: "yesspace", Name: "yesname"}
	req := reconcile.Request{NamespacedName: name}
	res, err := controller.Reconcile(ctx, req)
	require.NoError(t, err)
	require.True(t, res.IsZero())
}

type imagePullerMock struct {
	mock.Mock
}

func (m *imagePullerMock) Pull(ctx context.Context, image string) (packagecontent.Files, error) {
	args := m.Called(ctx, image)
	return args.Get(0).(packagecontent.Files), args.Error(1)
}

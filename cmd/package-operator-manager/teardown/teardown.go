package teardown

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	rbac "k8s.io/api/rbac/v1"
	ext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkocore "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/cmd/package-operator-manager/bootstrap"
	"package-operator.run/cmd/package-operator-manager/components"
	"package-operator.run/internal/controllers"
	"package-operator.run/internal/utils"
)

// Teardown is responsible for removing an existing package operator installation.
type Teardown struct{ c components.UncachedClient }

// NewTeardown creates a new [Teardown] with the given client.
func NewTeardown(client components.UncachedClient) *Teardown { return &Teardown{client} }

// Teardown performs the removal of an existing PKO installation.
func (t Teardown) Teardown(ctx context.Context) error {
	steps := []func(context.Context) error{t.removePackages, t.removeClusterPackages, t.removePKOClusterPackageFinalizer, t.removeClusterObjectSetFinalizer, t.removeCRDs, t.removePerms}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}

	logr.FromContextOrDiscard(ctx).Info("teardown complete")

	return nil
}

// removePackages removes all Packages (not ClusterPackages) and waits for them to be actually deleted.
func (t Teardown) removePackages(ctx context.Context) error {
	pl := pkocore.PackageList{}
	if err := t.c.List(ctx, &pl, &client.ListOptions{Namespace: ""}); err != nil {
		return fmt.Errorf("list packages: %w", err)
	}

	for i := range pl.Items {
		pkg := &pl.Items[i]

		logr.FromContextOrDiscard(ctx).Info("deleting package", "name", pkg.Name)

		if err := t.c.Delete(ctx, pkg); err != nil {
			return fmt.Errorf("delete package: %w", err)
		}
	}

	for i := range pl.Items {
		if err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, utils.ConditionFnNotFound(t.c, &pl.Items[i])); err != nil {
			return fmt.Errorf("wait for pkg to be gone: %w", err)
		}
	}

	return nil
}

// removeClusterPackages removes all ClusterPackages (not Packages) except the PKO ClusterPackage itself and waits for them to be gone.
func (t Teardown) removeClusterPackages(ctx context.Context) error {
	cpl := pkocore.ClusterPackageList{}
	if err := t.c.List(ctx, &cpl); err != nil {
		return fmt.Errorf("list clusterpackages: %w", err)
	}

	for i := range cpl.Items {
		pkg := &cpl.Items[i]

		if pkg.Name == bootstrap.ClusterPackageName {
			continue
		}
		logr.FromContextOrDiscard(ctx).Info("deleting cluster package", "name", &pkg.Name)

		if err := t.c.Delete(ctx, pkg); err != nil {
			return fmt.Errorf("delete clusterpackage: %w", err)
		}
	}

	for i := range cpl.Items {
		pkg := &cpl.Items[i]

		if pkg.Name == bootstrap.ClusterPackageName {
			continue
		}

		if err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, utils.ConditionFnNotFound(t.c, pkg)); err != nil {
			return fmt.Errorf("wait for crd to be gone: %w", err)
		}
	}

	return nil
}

// removePKOClusterPackageFinalizer removes the teardown finalizer from the PKO package.
//
// This allows it to be deleted when PKO managed CRDs are removed.
func (t Teardown) removePKOClusterPackageFinalizer(ctx context.Context) error {
	var pkg pkocore.ClusterPackage

	if err := t.c.Get(ctx, types.NamespacedName{Name: bootstrap.ClusterPackageName}, &pkg); err != nil {
		return fmt.Errorf("get PKO cluster package: %w", err)
	}

	if err := controllers.RemoveFinalizer(ctx, t.c, &pkg, bootstrap.ClusterPackageFinalizer); err != nil {
		return fmt.Errorf("remove finalizer from PKO cluster package: %w", err)
	}

	return nil
}

// removeClusterObjectSetFinalizer removes cache finalizer from ClusterObjectSets.
//
// PKO doesn't clean them up properly so we do that here.
func (t Teardown) removeClusterObjectSetFinalizer(ctx context.Context) error {
	cobsl := pkocore.ClusterObjectSetList{}
	if err := t.c.List(ctx, &cobsl); err != nil {
		return fmt.Errorf("list PKO clusterobjectsets: %w", err)
	}

	for i := range cobsl.Items {
		logr.FromContextOrDiscard(ctx).Info("remove cached finalizer from ClusterObjectSet", "name", &cobsl.Items[i].Name)

		if err := controllers.RemoveFinalizer(ctx, t.c, &cobsl.Items[i], "package-operator.run/cached"); err != nil {
			return fmt.Errorf("remove cached finalizer from clusterobjectset: %w", err)
		}
	}

	return nil
}

// removeCRDs removes CRDs that were installed with PKO and waits for them to be gone.
func (t Teardown) removeCRDs(ctx context.Context) error {
	selector, err := labels.ValidatedSelectorFromSet(map[string]string{"package-operator.run/package": "package-operator"})
	if err != nil {
		panic(err)
	}

	crdl := ext.CustomResourceDefinitionList{}

	if err := t.c.List(ctx, &crdl, &client.ListOptions{LabelSelector: selector}); err != nil {
		return fmt.Errorf("list crds: %w", err)
	}

	for i := range crdl.Items {
		crd := &crdl.Items[i]

		logr.FromContextOrDiscard(ctx).Info("deleting crd", "name", crd.Name)

		if err := t.c.Delete(ctx, crd); err != nil {
			return fmt.Errorf("delete crd: %w", err)
		}
	}

	for i := range crdl.Items {
		if err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, utils.ConditionFnNotFound(t.c, &crdl.Items[i])); err != nil {
			return fmt.Errorf("wait for crd to be gone: %w", err)
		}
	}

	return nil
}

// removePerms removes the ClusterRole and ClusterRoleBinding of PKO.
func (t Teardown) removePerms(ctx context.Context) error {
	if err := t.c.Delete(ctx, &rbac.ClusterRole{ObjectMeta: meta.ObjectMeta{Name: "package-operator-remote-phase-manager"}}); err != nil {
		return fmt.Errorf("delete PKO crb: %w", err)
	}

	if err := t.c.Delete(ctx, &rbac.ClusterRoleBinding{ObjectMeta: meta.ObjectMeta{Name: "package-operator"}}); err != nil {
		return fmt.Errorf("delete PKO crb: %w", err)
	}

	return nil
}
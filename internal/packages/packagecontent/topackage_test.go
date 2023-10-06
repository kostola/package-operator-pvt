package packagecontent_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"package-operator.run/internal/packages"
	"package-operator.run/internal/packages/packagecontent"
	"package-operator.run/internal/packages/packageimport"
	"package-operator.run/internal/testutil"
)

var (
	testDataPath = filepath.Join("testdata", "base")
	testManifest = "apiVersion: manifests.package-operator.run/v1alpha1\n" +
		"kind: PackageManifest\n" +
		"metadata:\n" +
		"  name: test\n" +
		"spec:\n" +
		"  scopes:\n" +
		"    - Namespaced\n" +
		"  phases:\n" +
		"    - name: configure"
)

func TestPackageFromFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	files, err := packageimport.Folder(ctx, testDataPath)
	require.NoError(t, err)

	pkg, err := packagecontent.PackageFromFiles(ctx, testutil.Scheme, files, "")
	require.NoError(t, err)
	require.NotNil(t, pkg)
}

func TestTemplateSpecFromPackage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	files, err := packageimport.Folder(ctx, testDataPath)
	require.NoError(t, err)

	pkg, err := packagecontent.PackageFromFiles(ctx, testutil.Scheme, files, "")
	require.NoError(t, err)
	require.NotNil(t, pkg)

	spec := packagecontent.TemplateSpecFromPackage(pkg)
	require.NotNil(t, spec)
}

func TestPackageManifestLoader_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fileMap packagecontent.Files
	}{
		{
			name: "not found",
		},
		{
			name: "invalid YAML",
			fileMap: packagecontent.Files{
				packages.PackageManifestFilename: []byte("{xxx..,akd:::"),
			},
		},
		{
			name: "invalid GVK",
			fileMap: packagecontent.Files{
				packages.PackageManifestFilename: []byte("apiVersion: fruits/v1\nkind: Banana"),
			},
		},
		{
			name: "unsupported Version",
			fileMap: packagecontent.Files{
				packages.PackageManifestFilename: []byte("apiVersion: manifests.package-operator.run/v23\nkind: PackageManifest"),
			},
		},
		{
			name: "multiple manifests",
			fileMap: packagecontent.Files{
				packages.PackageManifestFilename: []byte(testManifest),
				"manifest.yml":                   []byte(testManifest),
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := packagecontent.PackageFromFiles(context.Background(), testutil.Scheme, test.fileMap, "")
			require.Error(t, err)
		})
	}
}

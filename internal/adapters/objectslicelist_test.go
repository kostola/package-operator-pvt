package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "package-operator.run/apis/core/v1alpha1"
	"package-operator.run/internal/testutil"
)

func TestObjectSliceList(t *testing.T) {
	t.Parallel()

	sliceList := NewObjectSliceList(testutil.Scheme).(*ObjectSliceList)
	assert.IsType(t, &corev1alpha1.ObjectSliceList{}, sliceList.ClientObjectList())

	sliceList.Items = []corev1alpha1.ObjectSlice{
		{
			ObjectMeta: metav1.ObjectMeta{},
		},
	}
	items := sliceList.GetItems()
	if assert.Len(t, items, 1) {
		assert.IsType(t, &ObjectSlice{}, items[0])
	}
}

func TestClusterObjectSliceList(t *testing.T) {
	t.Parallel()

	sliceList := NewClusterObjectSliceList(testutil.Scheme).(*ClusterObjectSliceList)
	assert.IsType(t, &corev1alpha1.ClusterObjectSliceList{}, sliceList.ClientObjectList())

	sliceList.Items = []corev1alpha1.ClusterObjectSlice{
		{
			ObjectMeta: metav1.ObjectMeta{},
		},
	}
	items := sliceList.GetItems()
	if assert.Len(t, items, 1) {
		assert.IsType(t, &ClusterObjectSlice{}, items[0])
	}
}

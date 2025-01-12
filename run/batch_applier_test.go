package run

import (
	"fmt"
	"testing"

	"github.com/utilitywarehouse/kube-applier/kube"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/metrics"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type batchTestCase struct {
	ba        BatchApplier
	applyList []string

	expectedSuccesses []ApplyAttempt
	expectedFailures  []ApplyAttempt
}

func TestBatchApplierApply(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// Empty apply list
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
		},
		[]string{},
		[]ApplyAttempt{},
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplySuccess(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// All files succeed
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file1", kubeClient),
		expectApplyAndReturnSuccess("file1", "file1", false, true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", false, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file3", kubeClient),
		expectApplyAndReturnSuccess("file3", "file3", false, true, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplyFail(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// All files fail
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file1", kubeClient),
		expectApplyAndReturnFailure("file1", "file1", false, true, kubeClient),
		expectFailureMetric("file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnFailure("file2", "file2", false, true, kubeClient),
		expectFailureMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file3", kubeClient),
		expectApplyAndReturnFailure("file3", "file3", false, true, kubeClient),
		expectFailureMetric("file3", metrics),
	)
	failures := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", "error file1"},
		{"file2", "cmd file2", "output file2", "error file2"},
		{"file3", "cmd file3", "output file3", "error file3"},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
		},
		applyList,
		[]ApplyAttempt{},
		failures,
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplyPartial(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// Some successes, some failures
	applyList := []string{"file1", "file2", "file3", "file4"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file1", kubeClient),
		expectApplyAndReturnSuccess("file1", "file1", false, true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnFailure("file2", "file2", false, true, kubeClient),
		expectFailureMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file3", kubeClient),
		expectApplyAndReturnSuccess("file3", "file3", false, true, kubeClient),
		expectSuccessMetric("file3", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file4", kubeClient),
		expectApplyAndReturnFailure("file4", "file4", false, true, kubeClient),
		expectFailureMetric("file4", metrics),
	)
	successes := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	failures := []ApplyAttempt{
		{"file2", "cmd file2", "output file2", "error file2"},
		{"file4", "cmd file4", "output file4", "error file4"},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
		},
		applyList,
		successes,
		failures,
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplySuccessDryRun(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// All files succeed dry-run
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file1", kubeClient),
		expectApplyAndReturnSuccess("file1", "file1", true, true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", true, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file3", kubeClient),
		expectApplyAndReturnSuccess("file3", "file3", true, true, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
			DryRun:     true,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplySuccessDryRunNamespaces(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// All files succeed dry-run namespaces
	applyList := []string{"repo/file1", "file2", "repo/file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true", DryRun: "true"}, "file1", kubeClient),
		expectApplyAndReturnSuccess("repo/file1", "file1", true, true, kubeClient),
		expectSuccessMetric("repo/file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", false, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true", DryRun: "true"}, "file3", kubeClient),
		expectApplyAndReturnSuccess("repo/file3", "file3", true, true, kubeClient),
		expectSuccessMetric("repo/file3", metrics),
	)
	successes := []ApplyAttempt{
		{"repo/file1", "cmd repo/file1", "output repo/file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"repo/file3", "cmd repo/file3", "output repo/file3", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
			DryRun:     false,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplySuccessDryRunAndDryRunNamespaces(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// All files succeed dry-run and dry-run namespaces
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true", DryRun: "true"}, "file1", kubeClient),
		expectApplyAndReturnSuccess("file1", "file1", true, true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", true, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true", DryRun: "true"}, "file3", kubeClient),
		expectApplyAndReturnSuccess("file3", "file3", true, true, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
			DryRun:     true,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplyDisabledNamespaces(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	//Disabled namespaces
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "false"}, "file1", kubeClient),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", false, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "false"}, "file3", kubeClient),
	)
	successes := []ApplyAttempt{
		{"file2", "cmd file2", "output file2", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
			DryRun:     false,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func TestBatchApplierApplyInvalidAnnotation(t *testing.T) {
	log.InitLogger("info")
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	//Unsupported automatic deployment option on namespace
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "unsupportedOption"}, "file1", kubeClient),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "true"}, "file2", kubeClient),
		expectApplyAndReturnSuccess("file2", "file2", false, true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectNamespaceAnnotationsAndReturn(kube.KAAnnotations{Enabled: "unsupportedOption"}, "file3", kubeClient),
	)
	successes := []ApplyAttempt{
		{"file2", "cmd file2", "output file2", ""},
	}
	tc := batchTestCase{
		BatchApplier{
			KubeClient: kubeClient,
			Metrics:    metrics,
			DryRun:     false,
		},
		applyList,
		successes,
		[]ApplyAttempt{},
	}
	applyAndAssert(t, tc)
}

func expectApplyAndReturnSuccess(file, namespace string, dryRun, prune bool, kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().Apply(file, namespace, dryRun, prune, false).Times(1).Return("cmd "+file, "output "+file, nil)
}

func expectApplyAndReturnFailure(file, namespace string, dryRun, prune bool, kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().Apply(file, namespace, dryRun, prune, false).Times(1).Return("cmd "+file, "output "+file, fmt.Errorf("error "+file))
}

func expectNamespaceAnnotationsAndReturn(ret kube.KAAnnotations, namespace string, kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().NamespaceAnnotations(namespace).Times(1).Return(ret, nil)
}

func expectSuccessMetric(file string, metrics *metrics.MockPrometheusInterface) *gomock.Call {
	return metrics.EXPECT().UpdateNamespaceSuccess(file, true).Times(1)
}

func expectFailureMetric(file string, metrics *metrics.MockPrometheusInterface) *gomock.Call {
	return metrics.EXPECT().UpdateNamespaceSuccess(file, false).Times(1)
}

func applyAndAssert(t *testing.T, tc batchTestCase) {
	assert := assert.New(t)
	successes, failures := tc.ba.Apply(tc.applyList)
	assert.Equal(tc.expectedSuccesses, successes)
	assert.Equal(tc.expectedFailures, failures)
}

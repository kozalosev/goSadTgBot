package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestInitCounters(t *testing.T) {
	InitPanicsCounters("test_kind")
	InitUpdateKindCounters("test_kind")
	InitWizardCompletedCounters("TestWizard")

	// the series exist with zeros before the first increment
	assert.Zero(t, testutil.ToFloat64(panicsCounter.WithLabelValues("test_kind")))
	assert.Zero(t, testutil.ToFloat64(updateKindCounter.WithLabelValues("test_kind")))
	assert.Zero(t, testutil.ToFloat64(wizardCompletedCounter.WithLabelValues("TestWizard")))
	assert.Positive(t, testutil.CollectAndCount(panicsCounter, "panics_total"))
}

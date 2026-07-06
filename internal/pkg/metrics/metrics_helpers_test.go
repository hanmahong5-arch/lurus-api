package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// histogramSampleCount returns the cumulative sample_count of an unlabeled
// prometheus.Histogram. It reads the metric directly via the client_model dto
// so tests can assert the exact number of observations recorded (something
// testutil.ToFloat64 cannot do for histograms).
func histogramSampleCount(t *testing.T, h prometheus.Histogram) int {
	t.Helper()
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("histogram Write failed: %v", err)
	}
	hist := m.GetHistogram()
	if hist == nil {
		t.Fatalf("metric is not a histogram")
	}
	return int(hist.GetSampleCount())
}

// observerSampleCount returns the cumulative sample_count of a single
// prometheus.Observer time series (typically HistogramVec.WithLabelValues(...)).
// The Observer's concrete type also implements prometheus.Metric, so we
// type-assert to reach Write and read the histogram sample count.
func observerSampleCount(t *testing.T, o prometheus.Observer) int {
	t.Helper()
	metric, ok := o.(prometheus.Metric)
	if !ok {
		t.Fatalf("observer does not implement prometheus.Metric (%T)", o)
	}
	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		t.Fatalf("observer Write failed: %v", err)
	}
	hist := m.GetHistogram()
	if hist == nil {
		t.Fatalf("observer metric is not a histogram")
	}
	return int(hist.GetSampleCount())
}

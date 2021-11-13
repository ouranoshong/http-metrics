package httpmetrics

import (
	"net/http"
	"testing"
	"time"
)

func TestResultDoHttpRequest(t *testing.T) {

	req, _ := http.NewRequest("GET", TEST_HTTP, nil)
	m, _ := NewMetricResult(req).Do()

	if m.isTLS {
		t.Fatal("isTLS should be false")
	}

	if got, want := m.TLSHandshake, 0*time.Millisecond; got != want {
		t.Fatalf("TLSHandshake time of HTTP = %d, want %d", got, want)
	}

	durations := m.durations()
	delete(durations, "TLSHandshake")

	for k, d := range durations {
		if d <= 0*time.Millisecond {
			t.Fatalf("expect %s to be non-zero", k)
		}
	}
}

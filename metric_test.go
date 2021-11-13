package httpmetrics

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestMetricFormatter(t *testing.T) {
	m := Metric{
		DNSLookup:        100 * time.Millisecond,
		TCPConnection:    100 * time.Millisecond,
		TLSHandshake:     100 * time.Millisecond,
		ServerProcessing: 100 * time.Millisecond,
		ContentTransfer:  100 * time.Millisecond,

		NameLookup:    100 * time.Millisecond,
		Connect:       100 * time.Millisecond,
		Pretransfer:   100 * time.Millisecond,
		StartTransfer: 100 * time.Millisecond,
		Total:         100 * time.Millisecond,
	}

	want := `DNS lookup:         100 ms
TCP connection:     100 ms
TLS handshake:      100 ms
Server processing:  100 ms
Content transfer:   100 ms
Name Lookup:     100 ms
Connect:         100 ms
Pre Transfer:    100 ms
Start Transfer:  100 ms
Total:           100 ms
`
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%+v", m)
	if got := buf.String(); want != got {
		t.Fatalf("expect to be eq:\n\nwant:\n\n%s\ngot:\n\n%s\n", want, got)
	}
}

const (
	TEST_HTTP  = "http://example.com"
	TEST_HTTPS = "https://example.com"
)

func DefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
func DefaultClient() *http.Client {
	return &http.Client{
		Transport: DefaultTransport(),
	}
}

func NewRequest(t *testing.T, url string, m *Metric) *http.Request {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatal("NewRequest failed:", err)
	}

	ctx := WithHttpMertics(req.Context(), m)
	return req.WithContext(ctx)
}

func TestHTTPS(t *testing.T) {
	var m Metric
	req := NewRequest(t, TEST_HTTPS, &m)

	client := DefaultClient()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal("client.Do failed:", err)
	}

	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		t.Fatal("io.Copy failed:", err)
	}
	res.Body.Close()
	m.End(time.Now())

	if !m.isTLS {
		t.Fatal("isTLS should be true")
	}

	for k, d := range m.durations() {
		if d <= 0*time.Millisecond {
			t.Fatalf("expect %s to be non-zero", k)
		}
	}
}

func TestHTTP(t *testing.T) {
	var m Metric
	req := NewRequest(t, TEST_HTTP, &m)

	client := DefaultClient()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal("client.Do failed:", err)
	}

	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		t.Fatal("io.Copy failed:", err)
	}
	res.Body.Close()
	m.End(time.Now())

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

func TestKeepAlive(t *testing.T) {
	req1, err := http.NewRequest("GET", TEST_HTTPS, nil)
	if err != nil {
		t.Fatal("NewRequest failed:", err)
	}

	client := DefaultClient()
	res1, err := client.Do(req1)
	if err != nil {
		t.Fatal("Request failed:", err)
	}

	if _, err := io.Copy(ioutil.Discard, res1.Body); err != nil {
		t.Fatal("Copy body failed:", err)
	}
	res1.Body.Close()

	var m Metric
	req2 := NewRequest(t, TEST_HTTPS, &m)

	res2, err := client.Do(req2)
	if err != nil {
		t.Fatal("Request failed:", err)
	}

	if _, err := io.Copy(ioutil.Discard, res2.Body); err != nil {
		t.Fatal("Copy body failed:", err)
	}
	res2.Body.Close()
	m.End(time.Now())

	durations := []time.Duration{
		m.DNSLookup,
		m.TCPConnection,
		m.TLSHandshake,
	}

	for i, d := range durations {
		if got, want := d, 0*time.Millisecond; got != want {
			t.Fatalf("#%d expect %d to be eq %d", i, got, want)
		}
	}
}

package httpmetrics

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http/httptrace"
	"strings"
	"time"
)

type Metric struct {
	DNSLookup        time.Duration
	TCPConnection    time.Duration
	TLSHandshake     time.Duration
	ServerProcessing time.Duration
	ContentTransfer  time.Duration

	NameLookup    time.Duration
	Connect       time.Duration
	Pretransfer   time.Duration
	StartTransfer time.Duration
	Total         time.Duration

	dnsStart      time.Time
	dnsDone       time.Time
	tcpStart      time.Time
	tcpDone       time.Time
	tlsStart      time.Time
	tlsDone       time.Time
	serverStart   time.Time
	serverDone    time.Time
	transferStart time.Time
	transferDone  time.Time

	isTLS    bool
	isReused bool
}

func (m *Metric) durations() map[string]time.Duration {
	return map[string]time.Duration{
		"DNSLookup":        m.DNSLookup,
		"TCPConnection":    m.TCPConnection,
		"TLSHandshake":     m.TLSHandshake,
		"ServerProcessing": m.ServerProcessing,
		"ContentTransfer":  m.ContentTransfer,

		"NameLookup":    m.NameLookup,
		"Connect":       m.Connect,
		"Pretransfer":   m.Connect,
		"StartTransfer": m.StartTransfer,
		"Total":         m.Total,
	}
}

func (m Metric) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "DNS lookup:        %4d ms\n", int(m.DNSLookup/time.Millisecond))
			fmt.Fprintf(&buf, "TCP connection:    %4d ms\n", int(m.TCPConnection/time.Millisecond))
			fmt.Fprintf(&buf, "TLS handshake:     %4d ms\n", int(m.TLSHandshake/time.Millisecond))
			fmt.Fprintf(&buf, "Server processing: %4d ms\n", int(m.ServerProcessing/time.Millisecond))
			fmt.Fprintf(&buf, "Content transfer:  %4d ms\n", int(m.ContentTransfer/time.Millisecond))
			fmt.Fprintf(&buf, "Name Lookup:    %4d ms\n", int(m.NameLookup/time.Millisecond))
			fmt.Fprintf(&buf, "Connect:        %4d ms\n", int(m.Connect/time.Millisecond))
			fmt.Fprintf(&buf, "Pre Transfer:   %4d ms\n", int(m.Pretransfer/time.Millisecond))
			fmt.Fprintf(&buf, "Start Transfer: %4d ms\n", int(m.StartTransfer/time.Millisecond))
			fmt.Fprintf(&buf, "Total:          %4d ms\n", int(m.Total/time.Millisecond))
			io.WriteString(s, buf.String())
			return
		}
		fallthrough
	case 's', 'q':
		d := m.durations()
		list := make([]string, 0, len(d))

		for k, v := range d {
			list = append(list, fmt.Sprintf("%s: %d ms", k, v/time.Millisecond))
		}
		io.WriteString(s, strings.Join(list, ", "))
	}
}

func WithHttpMertics(ctx context.Context, r *Metric) context.Context {
	return withClientTrace(ctx, r)
}

func (m *Metric) End(t time.Time) {
	m.transferDone = t
	if m.dnsStart.IsZero() {
		return
	}
	m.ContentTransfer = m.transferDone.Sub(m.transferStart)
	m.Total = m.transferDone.Sub(m.dnsStart)
}

func withClientTrace(ctx context.Context, m *Metric) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSStart: func(i httptrace.DNSStartInfo) {
			m.dnsStart = time.Now()
		},
		DNSDone: func(i httptrace.DNSDoneInfo) {
			m.dnsDone = time.Now()
			m.DNSLookup = m.dnsDone.Sub(m.dnsStart)
			m.NameLookup = m.dnsDone.Sub(m.dnsStart)
		},
		ConnectStart: func(_, _ string) {
			m.tcpStart = time.Now()
			if m.dnsStart.IsZero() {
				m.dnsStart = m.tcpStart
				m.dnsDone = m.tcpStart
			}
		},
		ConnectDone: func(network, addr string, err error) {
			m.tcpDone = time.Now()
			m.TCPConnection = m.tcpDone.Sub(m.tcpStart)
			m.Connect = m.tcpDone.Sub(m.dnsStart)
		},
		TLSHandshakeStart: func() {
			m.isTLS = true
			m.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			m.tlsDone = time.Now()
			m.TLSHandshake = m.tlsDone.Sub(m.tlsStart)
			m.Pretransfer = m.tlsDone.Sub(m.dnsStart)
		},
		GotConn: func(i httptrace.GotConnInfo) {
			if i.Reused {
				m.isReused = true
			}
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			m.serverStart = time.Now()

			if m.dnsStart.IsZero() && m.tcpStart.IsZero() {
				now := m.serverStart

				m.dnsStart = now
				m.dnsDone = now
				m.tcpStart = now
				m.tcpDone = now
			}

			if m.isReused {
				now := m.serverStart

				m.dnsStart = now
				m.dnsDone = now
				m.tcpStart = now
				m.tcpDone = now
				m.tlsStart = now
				m.tlsDone = now
			}

			if m.isTLS {
				return
			}

			m.TLSHandshake = m.tcpDone.Sub(m.tcpDone)
			m.Pretransfer = m.Connect
		},

		GotFirstResponseByte: func() {
			m.serverDone = time.Now()

			m.ServerProcessing = m.serverDone.Sub(m.serverStart)
			m.StartTransfer = m.serverDone.Sub(m.dnsStart)

			m.transferStart = m.serverDone
		},
	})
}

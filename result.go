package httpmetrics

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

type Result struct {
	m Metric
	r *http.Request
}

func NewMetricResult(r *http.Request) Result {
	return Result{
		r: r,
	}
}

func (rs Result) Do() (Metric, error) {
	ctx := WithHttpMertics(rs.r.Context(), &rs.m)
	req := rs.r.WithContext(ctx)
	client := http.DefaultClient
	res, err := client.Do(req)
	if err != nil {
		return rs.m, err
	}
	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		return rs.m, err
	}
	res.Body.Close()
	rs.m.End(time.Now())
	return rs.m, err
}

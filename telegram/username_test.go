package telegram

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
)

func fakeResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func TestProcessResponse(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"ratelimit status", 429, `{}`, StatusRatelimit},
		{"ratelimit json r", 200, `{"r":"/"}`, StatusRatelimit},
		{"empty h = available", 200, `{"h":""}`, StatusAvailable},
		{"missing h = available", 200, `{}`, StatusAvailable},
		{"taken", 200, `{"h":"<span class=\"tm-section-header-status tm-status-taken\">Taken</span>"}`, StatusTaken},
		{"avail auction", 200, `{"h":"<span class=\"tm-section-header-status tm-status-avail\">Sale</span>"}`, StatusAuctioned},
		{"sold", 200, `{"h":"<span class=\"tm-section-header-status tm-status-unavail\">Sold</span>"}`, StatusSold},
		{"unknown no marker", 200, `{"h":"<div>something else</div>"}`, StatusUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := processResponse(fakeResp(c.status, c.body))
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestProcessResponseBadJSON(t *testing.T) {
	if _, err := processResponse(fakeResp(200, `not json`)); err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

// stubTransport lets us test CheckUsername without network.
type stubTransport struct {
	resp *http.Response
	err  error
}

func (s stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func TestCheckUsernameTaken(t *testing.T) {
	old := httpClient
	defer func() { httpClient = old }()
	httpClient = &http.Client{Transport: stubTransport{
		resp: fakeResp(200, `{"h":"<span class=\"tm-section-header-status tm-status-taken\">x</span>"}`),
	}}
	st, err := CheckUsername(context.Background(), "durov")
	if err != nil || st != StatusTaken {
		t.Fatalf("got %q,%v want Taken,nil", st, err)
	}
}

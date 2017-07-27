package action_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Akagi201/esalert/action"
	"github.com/Akagi201/esalert/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToActioner(t *testing.T) {
	m := map[string]interface{}{
		"type":   "http",
		"method": "get",
		"url":    "http://example.com",
		"body":   "wat",
	}
	a, err := action.ToActioner(m)
	assert.Nil(t, err)
	assert.Equal(t, &action.HTTP{Method: "get", URL: "http://example.com", Body: "wat"}, a.Actioner)

	m = map[string]interface{}{
		"type": "slack",
		"text": "foo",
	}
	a, err = action.ToActioner(m)
	assert.Nil(t, err)
	assert.Equal(t, &action.Slack{Text: "foo"}, a.Actioner)
}

func TestHTTPAction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/good", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	})
	s := httptest.NewServer(mux)

	h := &action.HTTP{
		Method: "GET",
		URL:    s.URL + "/good",
		Body:   "OHAI",
	}
	require.Nil(t, h.Do(context.Context{}))

	h.URL = s.URL + "/bad"
	require.NotNil(t, h.Do(context.Context{}))
}

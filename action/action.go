// Package action implements all the different actions an alert can take should
// its condition be found to be true
package action

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Akagi201/esalert/config"
	"github.com/Akagi201/esalert/context"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

// Actioner describes an action type. There all multiple action types, but they
// all simply attempt to perform one action and that's it
type Actioner interface {

	// Do takes in the alert context, and possibly returnes an error if the
	// action failed
	Do(context.Context) error
}

// Action is a wrapper around an Actioner which contains some type information
type Action struct {
	Type string
	Actioner
}

// ToActioner takes in some arbitrary data (hopefully a map[string]interface{},
// looks at its "type" key, and any other fields necessary based on that type,
// and returns an Actioner (or an error)
func ToActioner(in interface{}) (Action, error) {
	min, ok := in.(map[string]interface{})
	if !ok {
		return Action{}, errors.New("action definition is not an object")
	}

	var a Actioner
	typ, _ := min["type"].(string)
	typ = strings.ToLower(typ)
	switch typ {
	case "log":
		a = &Log{}
	case "http":
		a = &HTTP{}
	case "slack":
		a = &Slack{}
	default:
		return Action{}, fmt.Errorf("unknown action type: %q", typ)
	}

	if err := mapstructure.Decode(min, a); err != nil {
		return Action{}, err
	}
	return Action{Type: typ, Actioner: a}, nil
}

// Log is an action which does nothing but print a log message. Useful when
// testing alerts and you don't want to set up any actions yet
type Log struct {
	Message string `mapstructure:"message"`
}

// Do logs the Log's message. It doesn't actually need any context
func (l *Log) Do(_ context.Context) error {
	log.WithFields(log.Fields{
		"message": l.Message,
	}).Infoln("doing log action")
	return nil
}

// HTTP is an action which performs a single http request. If the request's
// response doesn't have a 2xx response code then it's considered an error
type HTTP struct {
	Method  string            `mapstructure:"method"`
	URL     string            `mapstructure:"url"`
	Headers map[string]string `mapstructure:"headers"`
	Body    string            `mapstructure:"body"`
}

// Do performs the actual http request. It doesn't need the alert context
func (h *HTTP) Do(_ context.Context) error {
	r, err := http.NewRequest(h.Method, h.URL, bytes.NewBufferString(h.Body))
	if err != nil {
		return err
	}

	if h.Headers != nil {
		for k, v := range h.Headers {
			r.Header.Set(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("non 2xx response code returned: %d", resp.StatusCode)
	}

	return nil
}

// OpsGenie submits an alert to an Slack endpoint
type Slack struct {
	Text string `json:"text" mapstructure:"text"`
}

// Do performs the actual trigger request to the Slack api
func (s *Slack) Do(c context.Context) error {
	if config.Opts.SlackWebhook == "" {
		return errors.New("Slack key not set in config")
	}

	if s.Text == "" {
		return errors.New("missing required field text in Slack")
	}

	bodyb, err := json.Marshal(&s)
	if err != nil {
		return err
	}

	r, err := http.NewRequest("POST", config.Opts.SlackWebhook, bytes.NewBuffer(bodyb))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

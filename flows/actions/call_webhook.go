package actions

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/flows/events"
)

func init() {
	RegisterType(TypeCallWebhook, func() flows.Action { return &CallWebhookAction{} })
}

// TypeCallWebhook is the type for the call webhook action
const TypeCallWebhook string = "call_webhook"

// CallWebhookAction can be used to call an external service. The body, header and url fields may be
// templates and will be evaluated at runtime. A [event:webhook_called] event will be created based on
// the results of the HTTP call. If this action has a `result_name`, then addtionally it will create
// a new result with that name. If the webhook returned valid JSON, that will be accessible
// through `extra` on the result.
//
//   {
//     "uuid": "8eebd020-1af5-431c-b943-aa670fc74da9",
//     "type": "call_webhook",
//     "method": "GET",
//     "url": "http://localhost:49998/?cmd=success",
//     "headers": {
//       "Authorization": "Token AAFFZZHH"
//     },
//     "result_name": "webhook"
//   }
//
// @action call_webhook
type CallWebhookAction struct {
	BaseAction
	onlineAction

	Method     string            `json:"method" validate:"required,http_method"`
	URL        string            `json:"url" validate:"required"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	ResultName string            `json:"result_name,omitempty"`
}

// Type returns the type of this action
func (a *CallWebhookAction) Type() string { return TypeCallWebhook }

// Validate validates our action is valid and has all the assets it needs
func (a *CallWebhookAction) Validate(assets flows.SessionAssets) error {
	if a.Body != "" && a.Method == "GET" {
		return fmt.Errorf("can't specify body if method is GET")
	}

	return nil
}

// Execute runs this action
func (a *CallWebhookAction) Execute(run flows.FlowRun, step flows.Step, log flows.EventLog) error {

	// substitute any variables in our url
	url, err := run.EvaluateTemplateAsString(a.URL, true)
	if err != nil {
		a.logError(err, log)
	}
	if url == "" {
		a.logError(fmt.Errorf("call_webhook URL evaluated to empty string, skipping"), log)
		return nil
	}

	method := strings.ToUpper(a.Method)
	body := a.Body

	// substitute any body variables
	if body != "" {
		body, err = run.EvaluateTemplateAsString(body, false)
		if err != nil {
			a.logError(err, log)
		}
	}

	// build our request
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		a.logError(err, log)
		return nil
	}

	// add the custom headers, substituting any template vars
	for key, value := range a.Headers {
		headerValue, err := run.EvaluateTemplateAsString(value, false)
		if err != nil {
			a.logError(err, log)
		}

		req.Header.Add(key, headerValue)
	}

	webhook, err := flows.MakeWebhookCall(run.Session(), req)

	if err != nil {
		a.logError(err, log)
	} else {
		a.log(events.NewWebhookCalledEvent(webhook), log)
		if a.ResultName != "" {
			a.saveWebhookResult(run, step, a.ResultName, webhook, log)
		}
	}

	return nil
}

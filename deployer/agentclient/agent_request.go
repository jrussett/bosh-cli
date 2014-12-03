package agentclient

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	bmhttpclient "github.com/cloudfoundry/bosh-micro-cli/deployer/httpclient"
)

type AgentRequestMessage struct {
	Method    string        `json:"method"`
	Arguments []interface{} `json:"arguments"`
	ReplyTo   string        `json:"reply_to"`
}

type agentRequest struct {
	uuid       string
	endpoint   string
	httpClient bmhttpclient.HTTPClient
}

func NewAgentRequest(endpoint string, httpClient bmhttpclient.HTTPClient, uuid string) agentRequest {
	return agentRequest{
		endpoint:   endpoint,
		httpClient: httpClient,
		uuid:       uuid,
	}
}

func (r agentRequest) Send(method string, arguments []interface{}, response Response) error {
	postBody := AgentRequestMessage{
		Method:    method,
		Arguments: arguments,
		ReplyTo:   r.uuid,
	}

	agentRequestJSON, err := json.Marshal(postBody)
	if err != nil {
		return bosherr.WrapError(err, "Marshaling agent request")
	}

	httpResponse, err := r.httpClient.Post(r.endpoint, agentRequestJSON)
	if err != nil {
		return bosherr.WrapErrorf(err, "Performing request to agent endpoint '%s'", r.endpoint)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return bosherr.Errorf("Agent responded with non-successful status code: %d", httpResponse.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return bosherr.WrapError(err, "Reading agent response")
	}

	err = response.Unmarshal(responseBody)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshaling agent response")
	}

	exception := response.GetException()

	if !exception.IsEmpty() {
		return bosherr.Errorf("Agent responded with error: %s", exception.Message)
	}

	return nil
}

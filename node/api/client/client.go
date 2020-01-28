package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"gitlab.com/NebulousLabs/Sia/node/api"
	"gitlab.com/NebulousLabs/errors"
)

// A Client makes requests to the siad HTTP API.
type Client struct {
	// Address is the API address of the siad server.
	Address string

	// Password must match the password of the siad server.
	Password string

	// UserAgent must match the User-Agent required by the siad server. If not
	// set, it defaults to "Sia-Agent".
	UserAgent string
}

// New creates a new Client using the provided address.
func New(address string) *Client {
	return &Client{
		Address: address,
	}
}

// NewRequest constructs a request to the siad HTTP API, setting the correct
// User-Agent and Basic Auth. The resource path must begin with /.
func (c *Client) NewRequest(method, resource string, body io.Reader) (*http.Request, error) {
	url := "http://" + c.Address + resource
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	agent := c.UserAgent
	if agent == "" {
		agent = "Sia-Agent"
	}
	req.Header.Set("User-Agent", agent)
	if c.Password != "" {
		req.SetBasicAuth("", c.Password)
	}
	return req, nil
}

// drainAndClose reads rc until EOF and then closes it. drainAndClose should
// always be called on HTTP response bodies, because if the body is not fully
// read, the underlying connection can't be reused.
func drainAndClose(rc io.ReadCloser) {
	io.Copy(ioutil.Discard, rc)
	rc.Close()
}

// readAPIError decodes and returns an api.Error.
func readAPIError(r io.Reader) error {
	var apiErr api.Error
	if err := json.NewDecoder(r).Decode(&apiErr); err != nil {
		return errors.AddContext(err, "could not read error response")
	}

	if strings.Contains(apiErr.Error(), ErrPeerExists.Error()) {
		return ErrPeerExists
	}

	return apiErr
}

// getRawResponse requests the specified resource. The response, if provided,
// will be returned in a byte slice
func (c *Client) getRawResponse(resource string) (http.Header, []byte, error) {
	req, err := c.NewRequest("GET", resource, nil)
	if err != nil {
		return nil, nil, errors.AddContext(err, "failed to construct GET request")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, errors.AddContext(err, "GET request failed")
	}
	defer drainAndClose(res.Body)

	if res.StatusCode == http.StatusNotFound {
		return nil, nil, errors.AddContext(api.ErrAPICallNotRecognized, "unable to perform GET on "+resource)
	}

	// If the status code is not 2xx, decode and return the accompanying
	// api.Error.
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, nil, errors.AddContext(readAPIError(res.Body), "GET request error")
	}

	if res.StatusCode == http.StatusNoContent {
		// no reason to read the response
		return res.Header, []byte{}, nil
	}
	d, err := ioutil.ReadAll(res.Body)
	return res.Header, d, err
}

// getReaderResponse requests the specified resource. The response, if provided,
// will be returned as an io.Reader.
func (c *Client) getReaderResponse(resource string) (http.Header, io.ReadCloser, error) {
	req, err := c.NewRequest("GET", resource, nil)
	if err != nil {
		return nil, nil, errors.AddContext(err, "failed to construct GET request")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, errors.AddContext(err, "GET request failed")
	}

	if res.StatusCode == http.StatusNotFound {
		drainAndClose(res.Body)
		return nil, nil, errors.AddContext(api.ErrAPICallNotRecognized, "unable to perform GET on "+resource)
	}

	// If the status code is not 2xx, decode and return the accompanying
	// api.Error.
	if res.StatusCode < 200 || res.StatusCode > 299 {
		drainAndClose(res.Body)
		return nil, nil, errors.AddContext(readAPIError(res.Body), "GET request error")
	}

	if res.StatusCode == http.StatusNoContent {
		// no reason to read the response
		drainAndClose(res.Body)
		return res.Header, nil, nil
	}
	return res.Header, res.Body, nil
}

// getRawResponse requests part of the specified resource. The response, if
// provided, will be returned in a byte slice
func (c *Client) getRawPartialResponse(resource string, from, to uint64) ([]byte, error) {
	req, err := c.NewRequest("GET", resource, nil)
	if err != nil {
		return nil, errors.AddContext(err, "failed to construct GET request")
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", from, to-1))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.AddContext(err, "GET request failed")
	}
	defer drainAndClose(res.Body)

	if res.StatusCode == http.StatusNotFound {
		return nil, errors.AddContext(api.ErrAPICallNotRecognized, "unable to perform GET on "+resource)
	}

	// If the status code is not 2xx, decode and return the accompanying
	// api.Error.
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.AddContext(readAPIError(res.Body), "GET request error")
	}

	if res.StatusCode == http.StatusNoContent {
		// no reason to read the response
		return []byte{}, nil
	}
	return ioutil.ReadAll(res.Body)
}

// get requests the specified resource. The response, if provided, will be
// decoded into obj. The resource path must begin with /.
func (c *Client) get(resource string, obj interface{}) error {
	// Request resource
	_, data, err := c.getRawResponse(resource)
	if err != nil {
		return err
	}
	if obj == nil {
		// No need to decode response
		return nil
	}

	// Decode response
	buf := bytes.NewBuffer(data)
	err = json.NewDecoder(buf).Decode(obj)
	if err != nil {
		return errors.AddContext(err, "could not read response")
	}
	return nil
}

// postRawResponse requests the specified resource. The response, if provided,
// will be returned in a byte slice
func (c *Client) postRawResponse(resource string, body io.Reader) ([]byte, error) {
	req, err := c.NewRequest("POST", resource, body)
	if err != nil {
		return nil, errors.AddContext(err, "failed to construct POST request")
	}
	// TODO: is this necessary?
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.AddContext(err, "POST request failed")
	}
	defer drainAndClose(res.Body)

	if res.StatusCode == http.StatusNotFound {
		return nil, errors.AddContext(api.ErrAPICallNotRecognized, "unable to perform POST on "+resource)
	}

	// If the status code is not 2xx, decode and return the accompanying
	// api.Error.
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.AddContext(readAPIError(res.Body), "POST request error")
	}

	if res.StatusCode == http.StatusNoContent {
		// no reason to read the response
		return []byte{}, nil
	}
	d, err := ioutil.ReadAll(res.Body)
	return d, err
}

// post makes a POST request to the resource at `resource`, using `data` as the
// request body. The response, if provided, will be decoded into `obj`.
func (c *Client) post(resource string, data string, obj interface{}) error {
	// Request resource
	body, err := c.postRawResponse(resource, strings.NewReader(data))
	if err != nil {
		return err
	}
	if obj == nil {
		// No need to decode response
		return nil
	}

	// Decode response
	buf := bytes.NewBuffer(body)
	err = json.NewDecoder(buf).Decode(obj)
	if err != nil {
		return errors.AddContext(err, "could not read response")
	}
	return nil
}

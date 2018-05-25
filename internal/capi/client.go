package capi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

type Client struct {
	addr string
	doer Doer
}

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewClient(addr string, d Doer) *Client {
	return &Client{
		doer: d,
		addr: addr,
	}
}

func (c *Client) CreateTask(
	command string,
	name string,
	appGuid string,
) error {
	u, err := url.Parse(c.addr)
	if err != nil {
		return err
	}
	u.Path = fmt.Sprintf("/v3/apps/%s/tasks", appGuid)

	marshalled, err := json.Marshal(struct {
		Command string `json:"command"`
	}{
		Command: command,
	})
	if err != nil {
		return err
	}

	req := &http.Request{
		URL:    u,
		Method: "POST",
		Body:   ioutil.NopCloser(bytes.NewReader(marshalled)),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != 202 {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, data)
	}

	return nil
}

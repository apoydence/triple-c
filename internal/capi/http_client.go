package capi

import (
	"net/http"
)

type HTTPClient struct {
	doer Doer

	tokenFetcher TokenFetcher
	lastToken    string
}

type TokenFetcher interface {
	GetToken() (string, error)
}

type TokenFetcherFunc func() (string, error)

func (f TokenFetcherFunc) GetToken() (string, error) {
	return f()
}

func NewHTTPClient(doer Doer, f TokenFetcher) *HTTPClient {
	return &HTTPClient{
		doer:         doer,
		tokenFetcher: f,
	}
}

func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	for i := 0; i < 2; i++ {
		token, err := c.fetchToken()
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", token)

		resp, err = c.doer.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode > 299 {
			c.lastToken = ""
			continue
		}

		break
	}
	return resp, nil
}

func (c *HTTPClient) fetchToken() (string, error) {
	if c.lastToken != "" {
		return c.lastToken, nil
	}

	token, err := c.tokenFetcher.GetToken()
	if err != nil {
		return "", err
	}

	c.lastToken = token
	return token, nil
}

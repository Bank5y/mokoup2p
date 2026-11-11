package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Bank5y/mokoup2p/api"
	"github.com/Bank5y/mokoup2p/client/ipc"
)

// serverClient calls the server HTTP API.
type serverClient struct {
	http *http.Client
}

func newServerClient() *serverClient {
	return &serverClient{http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *serverClient) login(ctx context.Context, baseURL string, req api.LoginRequest) (api.Session, error) {
	var sess api.Session
	err := c.post(ctx, baseURL+api.PathLogin, req, &sess)
	return sess, err
}

func (c *serverClient) refresh(ctx context.Context, baseURL, refreshToken string) (api.Session, error) {
	var sess api.Session
	err := c.post(ctx, baseURL+api.PathRefresh, api.RefreshRequest{RefreshToken: refreshToken}, &sess)
	return sess, err
}

func (c *serverClient) post(ctx context.Context, url string, in, out any) error {
	return c.send(ctx, http.MethodPost, url, "", in, out)
}

// send makes a request, authenticated if accessToken is set. in and out may
// be nil. It returns *api.Error for server-reported failures,
// ipc.ErrServerUnreachable when the server cannot be reached, and
// ipc.ErrServerBadResponse when something answered that is not a mokoup2p
// server (e.g. a proxy error page).
func (c *serverClient) send(ctx context.Context, method, url, accessToken string, in, out any) error {
	var body io.Reader = http.NoBody
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &api.Error{Code: ipc.ErrServerUnreachable, Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		apiErr := &api.Error{}
		if err := json.NewDecoder(resp.Body).Decode(apiErr); err != nil || apiErr.Code == "" {
			return &api.Error{Code: ipc.ErrServerBadResponse, Message: fmt.Sprintf("unexpected response status %d", resp.StatusCode)}
		}
		return apiErr
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &api.Error{Code: ipc.ErrServerBadResponse, Message: "invalid response from server"}
	}
	return nil
}

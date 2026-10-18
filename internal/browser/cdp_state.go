package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type cdpConnection struct {
	conn *websocket.Conn
	id   int
}

func (c *cdpConnection) call(method string, params any, result any) error {
	c.id++
	if err := c.conn.WriteJSON(map[string]any{"id": c.id, "method": method, "params": params}); err != nil {
		return fmt.Errorf("could not send browser snapshot request")
	}
	for {
		var message struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := c.conn.ReadJSON(&message); err != nil {
			return fmt.Errorf("browser disconnected or snapshot timed out; nothing was uploaded")
		}
		if message.ID != c.id {
			continue
		}
		if message.Error != nil {
			return fmt.Errorf("browser refused snapshot operation %s", method)
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("invalid browser snapshot response")
		}
		return nil
	}
}

const readWebStorage = `(() => JSON.stringify({origin:location.origin,local:Object.fromEntries(Object.entries(localStorage)),session:Object.fromEntries(Object.entries(sessionStorage))}))()`

// ReadCDPState reads live first-party contexts without navigating or modifying
// pages. Distinct tab sessions for one origin cannot fit the cloud profile
// schema: conflicting values fail rather than silently selecting an account.
func ReadCDPState(ctx context.Context, endpoint string, domains []string) (State, error) {
	state := State{LocalStorage: map[string]map[string]string{}, SessionStorage: map[string]map[string]string{}}
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil {
		return State{}, fmt.Errorf("invalid --cdp-url")
	}
	var targets []struct {
		Type      string `json:"type"`
		URL       string `json:"url"`
		WebSocket string `json:"webSocketDebuggerUrl"`
	}
	if u.Scheme == "ws" || u.Scheme == "wss" {
		targets = append(targets, struct {
			Type      string `json:"type"`
			URL       string `json:"url"`
			WebSocket string `json:"webSocketDebuggerUrl"`
		}{Type: "page", WebSocket: endpoint})
	} else {
		if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
			return State{}, fmt.Errorf("use a CDP HTTP origin or page WebSocket URL")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/json/list", nil)
		if err != nil {
			return State{}, err
		}
		client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		if err != nil {
			return State{}, fmt.Errorf("cannot connect to browser debugger; enable remote debugging in a dedicated browser profile")
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return State{}, fmt.Errorf("browser debugger did not return a target list")
		}
		if err := json.NewDecoder(http.MaxBytesReader(nil, response.Body, MaxStateBytes)).Decode(&targets); err != nil {
			return State{}, fmt.Errorf("invalid browser target list")
		}
	}
	seenCookies := map[string]Cookie{}
	pages := 0
	for _, target := range targets {
		if target.Type != "page" || target.WebSocket == "" {
			continue
		}
		if target.URL != "" {
			if _, ok := storageOrigin(target.URL, domains); !ok {
				// Target URLs include paths, but storageOrigin deliberately accepts
				// origins only. Normalize the URL before selecting the page.
				pageURL, e := url.Parse(target.URL)
				if e != nil {
					continue
				}
				pageURL.Path = ""
				pageURL.RawQuery = ""
				pageURL.Fragment = ""
				pageURL.RawPath = ""
				if _, ok = storageOrigin(pageURL.String(), domains); !ok {
					continue
				}
			}
		}
		pages++
		if err := readTarget(ctx, target.WebSocket, domains, &state, seenCookies); err != nil {
			return State{}, err
		}
	}
	if pages == 0 {
		return State{}, fmt.Errorf("no open HTTP(S) page matches --domain; open the signed-in site and retry")
	}
	for _, cookie := range seenCookies {
		state.Cookies = append(state.Cookies, cookie)
	}
	return state, checkStateSize(state)
}

func readTarget(ctx context.Context, endpoint string, domains []string, state *State, cookies map[string]Cookie) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return fmt.Errorf("cannot connect to browser page debugger")
	}
	defer func() { _ = conn.Close() }()
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopCancellation()
	deadline := time.Now().Add(30 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	conn.SetReadLimit(MaxStateBytes)
	c := cdpConnection{conn: conn}
	var cookieResult struct {
		Cookies []struct {
			Name         string          `json:"name"`
			Value        string          `json:"value"`
			Domain       string          `json:"domain"`
			Path         string          `json:"path"`
			Secure       bool            `json:"secure"`
			HTTPOnly     bool            `json:"httpOnly"`
			SameSite     string          `json:"sameSite"`
			Expires      float64         `json:"expires"`
			Session      bool            `json:"session"`
			PartitionKey json.RawMessage `json:"partitionKey"`
		} `json:"cookies"`
	}
	if err := c.call("Network.getAllCookies", map[string]any{}, &cookieResult); err != nil {
		return err
	}
	for _, raw := range cookieResult.Cookies {
		if len(raw.PartitionKey) > 0 && string(raw.PartitionKey) != "null" {
			continue
		}
		if !domainMatches(raw.Domain, normalizeDomains(domains)) {
			continue
		}
		if !raw.Session && raw.Expires > 0 && raw.Expires <= float64(time.Now().Unix()) {
			continue
		}
		cookie := Cookie{Name: raw.Name, Value: raw.Value, Domain: raw.Domain, Path: raw.Path, Secure: raw.Secure, HTTPOnly: raw.HTTPOnly, SameSite: raw.SameSite, Expires: raw.Expires, Session: raw.Session}
		key := raw.Domain + "\x00" + raw.Path + "\x00" + raw.Name
		if prior, ok := cookies[key]; ok && prior != cookie {
			return fmt.Errorf("open browser contexts have conflicting cookies; select a single account and retry")
		}
		cookies[key] = cookie
	}
	var evaluated struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := c.call("Runtime.evaluate", map[string]any{"expression": readWebStorage, "returnByValue": true}, &evaluated); err != nil {
		return err
	}
	if len(evaluated.Exception) > 0 {
		return fmt.Errorf("browser storage is unavailable on a selected page")
	}
	var snapshot struct {
		Origin  string            `json:"origin"`
		Local   map[string]string `json:"local"`
		Session map[string]string `json:"session"`
	}
	if err := json.Unmarshal([]byte(evaluated.Result.Value), &snapshot); err != nil {
		return fmt.Errorf("invalid browser storage snapshot")
	}
	origin, allowed := storageOrigin(snapshot.Origin, domains)
	if !allowed {
		return fmt.Errorf("selected page changed origin during sync; retry")
	}
	for _, item := range []struct {
		target map[string]map[string]string
		source map[string]string
	}{{state.LocalStorage, snapshot.Local}, {state.SessionStorage, snapshot.Session}} {
		if prior, ok := item.target[origin]; ok && !maps.Equal(prior, item.source) {
			return fmt.Errorf("open tabs have conflicting storage for one origin; select a single account/tab and retry")
		}
		item.target[origin] = item.source
	}
	return nil
}

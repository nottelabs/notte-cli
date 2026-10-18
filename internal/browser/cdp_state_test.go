package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func cdpFixture(t *testing.T, sessions []string, fail bool) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []map[string]string{}
			for i := range sessions {
				targets = append(targets, map[string]string{"type": "page", "url": fmt.Sprintf("https://bsky.app/feed/%d", i), "webSocketDebuggerUrl": strings.Replace(server.URL, "http:", "ws:", 1) + fmt.Sprintf("/page/%d", i)})
			}
			targets = append(targets, map[string]string{"type": "page", "url": "https://unrelated.test/account", "webSocketDebuggerUrl": "ws://127.0.0.1:1/not-allowed"})
			_ = json.NewEncoder(w).Encode(targets)
			return
		}
		var index int
		if _, err := fmt.Sscanf(r.URL.Path, "/page/%d", &index); err != nil {
			t.Error(err)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			var req struct {
				ID     int            `json:"id"`
				Method string         `json:"method"`
				Params map[string]any `json:"params"`
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if err := conn.WriteJSON(map[string]any{"method": "Network.event", "params": map[string]any{}}); err != nil {
				return
			}
			if fail {
				_ = conn.WriteJSON(map[string]any{"id": req.ID, "error": map[string]any{"code": -1, "message": "synthetic-private-token"}})
				return
			}
			var result any
			switch req.Method {
			case "Network.getAllCookies":
				result = map[string]any{"cookies": []map[string]any{{"name": "login", "value": "synthetic-cookie", "domain": "bsky.app", "path": "/", "session": true}, {"name": "other", "value": "unrelated-private", "domain": "unrelated.test", "path": "/"}, {"name": "partitioned", "value": "excluded", "domain": "bsky.app", "path": "/", "partitionKey": map[string]any{"topLevelSite": "https://other.test"}}}}
			case "Runtime.evaluate":
				if req.Params["expression"] != readWebStorage {
					t.Error("snapshot must only read storage")
				}
				snapshot, _ := json.Marshal(map[string]any{"origin": "https://bsky.app", "local": map[string]string{"BSKY_STORAGE": "synthetic-local-login"}, "session": map[string]string{"tab": sessions[index]}})
				result = map[string]any{"result": map[string]any{"type": "string", "value": string(snapshot)}}
			default:
				t.Error("unexpected mutating CDP command", req.Method)
				return
			}
			if err := conn.WriteJSON(map[string]any{"id": req.ID, "result": result}); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestLiveStateFiltersAndDeduplicatesWithoutMutatingPages(t *testing.T) {
	s := cdpFixture(t, []string{"same-tab-state", "same-tab-state"}, false)
	state, err := ReadCDPState(context.Background(), s.URL, []string{"bsky.app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Cookies) != 1 || state.Cookies[0].Name != "login" || state.SessionStorage["https://bsky.app"]["tab"] != "same-tab-state" {
		t.Fatal("incorrect cookie or storage scope")
	}
}

func TestLiveStateRefusesConflictingTabSessions(t *testing.T) {
	s := cdpFixture(t, []string{"account-one", "account-two"}, false)
	state, err := ReadCDPState(context.Background(), s.URL, []string{"bsky.app"})
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatal("ambiguous account state must not be silently selected")
	}
	if len(state.LocalStorage) != 0 {
		t.Fatal("partial snapshot escaped after failure")
	}
}

func TestLiveStateFailureDoesNotExposeBrowserValues(t *testing.T) {
	s := cdpFixture(t, []string{"state"}, true)
	_, err := ReadCDPState(context.Background(), s.URL, []string{"bsky.app"})
	if err == nil || strings.Contains(err.Error(), "synthetic-private-token") {
		t.Fatal("browser error leaked private data")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = ReadCDPState(ctx, "http://127.0.0.1:1", nil)
	if err == nil {
		t.Fatal("unreachable browser accepted")
	}
}

func TestLiveStateRefusesInvalidEndpoints(t *testing.T) {
	for _, endpoint := range []string{"file:///etc/passwd", "http://user:password@localhost:9222", "http://localhost:9222/?secret=x"} {
		if _, err := ReadCDPState(context.Background(), endpoint, nil); err == nil {
			t.Fatal("invalid endpoint accepted")
		}
	}
}

func TestLiveStateCancellationInterruptsBlockedRead(t *testing.T) {
	connected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{}
		conn, err := u.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		close(connected)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := ReadCDPState(ctx, strings.Replace(server.URL, "http:", "ws:", 1), nil); done <- err }()
	<-connected
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled read succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not interrupt browser snapshot")
	}
}

func TestConcurrentLiveSnapshotsRemainIndependent(t *testing.T) {
	s := cdpFixture(t, []string{"same"}, false)
	var wait sync.WaitGroup
	for range 25 {
		wait.Go(func() {
			state, err := ReadCDPState(context.Background(), s.URL, []string{"bsky.app"})
			if err != nil || state.LocalStorage["https://bsky.app"]["BSKY_STORAGE"] != "synthetic-local-login" {
				t.Error("concurrent snapshot failed", err)
			}
		})
	}
	wait.Wait()
}

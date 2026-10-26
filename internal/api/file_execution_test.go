package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestExecuteSessionFileRequest(t *testing.T) {
	t.Parallel()
	id, origin, version := "B1", "cli", "test-version"
	record, updateMetadata := false, true
	var upload, download FileExecutionRequest_Action
	if err := upload.FromUploadFileActionInput(UploadFileActionInput{Id: &id, FilePath: "uploads/report.pdf"}); err != nil {
		t.Fatal(err)
	}
	if err := download.FromDownloadFileActionInput(DownloadFileActionInput{Id: &id}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []struct {
		name string
		body FileExecutionRequest_Action
		want map[string]any
	}{
		{"upload", upload, map[string]any{"type": "upload_file", "id": id, "file_path": "uploads/report.pdf"}},
		{"download", download, map[string]any{"type": "download_file", "id": id}},
	} {
		for _, options := range []struct {
			name   string
			params *ExecuteSessionFileParams
			record *bool
			query  string
		}{
			{"defaults", nil, nil, ""},
			{"explicit false", &ExecuteSessionFileParams{UpdateMetadata: &record}, &record, "update_metadata=false"},
			{"metadata and headers", &ExecuteSessionFileParams{
				UpdateMetadata: &updateMetadata, XNotteRequestOrigin: &origin, XNotteSdkVersion: &version,
			}, &record, "update_metadata=true"},
		} {
			t.Run(action.name+"/"+options.name, func(t *testing.T) {
				t.Parallel()
				request, err := NewExecuteSessionFileRequest("https://example.com/", "sess_123", options.params,
					ExecuteSessionFileJSONRequestBody{Action: action.body, TargetId: "page_123", Record: options.record})
				if err != nil {
					t.Fatal(err)
				}
				defer request.Body.Close()
				if request.Method != http.MethodPost || request.URL.Path != "/sessions/sess_123/page/execute/file" {
					t.Errorf("unexpected request: %s %s", request.Method, request.URL)
				}
				if request.URL.RawQuery != options.query {
					t.Errorf("query = %q, want %q", request.URL.RawQuery, options.query)
				}
				if got := request.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				for header, value := range map[string]*string{
					"x-notte-request-origin": nil, "x-notte-sdk-version": nil,
				} {
					if options.params != nil {
						if header == "x-notte-request-origin" {
							value = options.params.XNotteRequestOrigin
						} else {
							value = options.params.XNotteSdkVersion
						}
					}
					want := ""
					if value != nil {
						want = *value
					}
					if got := request.Header.Get(header); got != want {
						t.Errorf("%s = %q, want %q", header, got, want)
					}
				}
				var got map[string]any
				if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				want := map[string]any{"action": action.want, "target_id": "page_123"}
				if options.record != nil {
					want["record"] = *options.record
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("body = %#v, want %#v", got, want)
				}
			})
		}
	}
}

func TestExecuteSessionFileResponses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		body   string
		bad    bool
	}{
		{"success", 200, `{"success":true,"message":"Downloaded","action":{"type":"download_file","id":"B1"},"started_at":"2026-10-19T10:00:00Z","ended_at":"2026-10-19T10:00:01Z"}`, false},
		{"validation error", 422, `{"detail":[{"loc":["body","target_id"],"msg":"Field required","type":"missing"}]}`, false},
		{"server error", 500, `{"detail":"Internal server error"}`, false},
		{"malformed success", 200, `{`, true},
		{"malformed validation error", 422, `{`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := NewClientWithResponses(server.URL, WithHTTPClient(server.Client()))
			if err != nil {
				t.Fatal(err)
			}
			var action FileExecutionRequest_Action
			if err := action.FromDownloadFileActionInput(DownloadFileActionInput{}); err != nil {
				t.Fatal(err)
			}
			response, err := client.ExecuteSessionFileWithResponse(context.Background(), "sess_123", nil,
				ExecuteSessionFileJSONRequestBody{Action: action, TargetId: "page_123"})
			if tc.bad {
				if err == nil {
					t.Fatal("expected malformed JSON to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode() != tc.status || string(response.Body) != tc.body {
				t.Fatalf("response status/body = %d %s, want %d %s", response.StatusCode(), response.Body, tc.status, tc.body)
			}
			switch tc.status {
			case http.StatusOK:
				if response.JSON200 == nil || !response.JSON200.Success || response.JSON200.Message != "Downloaded" {
					t.Fatalf("unexpected success response: %+v", response.JSON200)
				}
				action, err := response.JSON200.Action.AsDownloadFileActionOutput()
				if err != nil || action.Id == nil || *action.Id != "B1" {
					t.Fatalf("lost response action: %+v, %v", action, err)
				}
			case http.StatusUnprocessableEntity:
				if response.JSON422 == nil || response.JSON422.Detail == nil || len(*response.JSON422.Detail) != 1 || (*response.JSON422.Detail)[0].Msg != "Field required" {
					t.Fatalf("unexpected validation response: %+v", response.JSON422)
				}
			default:
				if response.JSON200 != nil || response.JSON422 != nil {
					t.Fatal("server error decoded as a success or validation response")
				}
			}
		})
	}
}

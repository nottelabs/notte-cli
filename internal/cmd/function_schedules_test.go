package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const schedulesListJSON = `{"schedules":[` +
	`{"schedule_id":"` + functionIDTest + `","cron":"0 9 ? * MON-FRI *","variables":{"city":"paris"}},` +
	`{"schedule_id":"s2","cron":"0 17 ? * MON-FRI *","variables":{"city":"tokyo","limit":5}}` +
	`],"state":"enabled","paused_reason":null}`

// scheduleCommand is a command with the flags of add or update, set from flags.
func scheduleCommand(t *testing.T, register func(*cobra.Command), flags map[string][]string) *cobra.Command {
	t.Helper()
	origVars, origJSON := functionScheduleVariables, functionScheduleVariablesJSON
	origAdd, origUpdate := FunctionSchedulesAddCron, FunctionSchedulesUpdateCron
	functionScheduleVariables, functionScheduleVariablesJSON = []string{}, ""
	t.Cleanup(func() {
		functionScheduleVariables, functionScheduleVariablesJSON = origVars, origJSON
		FunctionSchedulesAddCron, FunctionSchedulesUpdateCron = origAdd, origUpdate
	})
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	register(cmd)
	cmd.Flags().StringArrayVar(&functionScheduleVariables, "var", []string{}, "")
	cmd.Flags().StringVar(&functionScheduleVariablesJSON, "vars", "", "")
	for name, values := range flags {
		for _, v := range values {
			if err := cmd.Flags().Set(name, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	return cmd
}

func sentBody(t *testing.T, server *testutil.MockServer, path, method string) map[string]any {
	t.Helper()
	for _, r := range server.Requests(path) {
		if r.Method == method {
			var body map[string]any
			if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
				t.Fatalf("request body %q: %v", r.Body, err)
			}
			return body
		}
	}
	t.Fatalf("no %s %s request", method, path)
	return nil
}

func TestRunFunctionSchedulesAddSendsCronAndVariables(t *testing.T) {
	server := setupFunctionTest(t)
	path := "/functions/" + functionIDTest + "/schedules"
	server.AddResponse(path, 200, `{"schedule_id":"s3","cron":"0 6 1 * ? *","variables":{"city":"berlin","limit":"3"}}`)
	cmd := scheduleCommand(t, RegisterFunctionSchedulesAddFlags, map[string][]string{
		"cron": {"0 6 1 * ? *"}, "vars": {`{"city": "rome", "limit": 3}`}, "var": {"city=berlin"},
	})

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runFunctionSchedulesAdd(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	body := sentBody(t, server, path, "POST")
	want := map[string]any{"cron": "0 6 1 * ? *", "variables": map[string]any{"city": "berlin", "limit": float64(3)}}
	if got, _ := json.Marshal(body); string(got) != mustJSON(t, want) {
		t.Errorf("body = %s, want %s", got, mustJSON(t, want))
	}
	if !strings.Contains(stdout, "s3") {
		t.Errorf("expected the new schedule in the output, got %q", stdout)
	}
}

func TestRunFunctionSchedulesAddWithoutVariablesSendsAnEmptyObject(t *testing.T) {
	server := setupFunctionTest(t)
	path := "/functions/" + functionIDTest + "/schedules"
	server.AddResponse(path, 200, `{"schedule_id":"s3","cron":"rate(1 hour)","variables":{}}`)
	cmd := scheduleCommand(t, RegisterFunctionSchedulesAddFlags, map[string][]string{"cron": {"rate(1 hour)"}})

	_, _ = testutil.CaptureOutput(func() {
		if err := runFunctionSchedulesAdd(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if got := sentBody(t, server, path, "POST")["variables"]; got == nil || len(got.(map[string]any)) != 0 {
		t.Errorf("variables = %v, want {}", got)
	}
}

func TestRunFunctionSchedulesUpdateKeepsWhatIsNotGiven(t *testing.T) {
	cases := []struct {
		name  string
		flags map[string][]string
		want  map[string]any
	}{
		{"cron only", map[string][]string{"cron": {"0 18 ? * MON-FRI *"}},
			map[string]any{"cron": "0 18 ? * MON-FRI *", "variables": map[string]any{"city": "tokyo", "limit": float64(5)}}},
		{"variables only", map[string][]string{"var": {"city=osaka"}},
			map[string]any{"cron": "0 17 ? * MON-FRI *", "variables": map[string]any{"city": "osaka"}}},
		{"both", map[string][]string{"cron": {"0 1 ? * * *"}, "vars": {`{"city": "kyoto"}`}},
			map[string]any{"cron": "0 1 ? * * *", "variables": map[string]any{"city": "kyoto"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := setupFunctionTest(t)
			server.AddResponse("/functions/"+functionIDTest+"/schedules", 200, schedulesListJSON)
			path := "/functions/" + functionIDTest + "/schedules/s2"
			server.AddResponse(path, 200, `{"schedule_id":"s2","cron":"0 18 ? * MON-FRI *","variables":{}}`)
			cmd := scheduleCommand(t, RegisterFunctionSchedulesUpdateFlags, c.flags)

			_, _ = testutil.CaptureOutput(func() {
				if err := runFunctionSchedulesUpdate(cmd, []string{"s2"}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			body := sentBody(t, server, path, "PATCH")
			if got, _ := json.Marshal(body); string(got) != mustJSON(t, c.want) {
				t.Errorf("body = %s, want %s", got, mustJSON(t, c.want))
			}
		})
	}
}

func TestRunFunctionSchedulesUpdateRejectsNoChange(t *testing.T) {
	setupFunctionTest(t)
	cmd := scheduleCommand(t, RegisterFunctionSchedulesUpdateFlags, nil)
	err := runFunctionSchedulesUpdate(cmd, []string{"s2"})
	if err == nil || !strings.Contains(err.Error(), "nothing to update") {
		t.Fatalf("err = %v, want nothing to update", err)
	}
}

func TestRunFunctionSchedulesUpdateUnknownSchedule(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/schedules", 200, schedulesListJSON)
	cmd := scheduleCommand(t, RegisterFunctionSchedulesUpdateFlags, map[string][]string{"cron": {"0 1 ? * * *"}})
	err := runFunctionSchedulesUpdate(cmd, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "schedule missing not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestRunFunctionSchedulesRemove(t *testing.T) {
	server := setupFunctionTest(t)
	path := "/functions/" + functionIDTest + "/schedules/s2"
	server.AddResponse(path, 200, `{"status":"DELETED"}`)
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runFunctionSchedulesRemove(cmd, []string{"s2"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if reqs := server.Requests(path); len(reqs) != 1 || reqs[0].Method != "DELETE" {
		t.Fatalf("requests = %+v, want one DELETE", reqs)
	}
	if !strings.Contains(stdout, "Schedule s2 removed") {
		t.Errorf("expected removal message, got %q", stdout)
	}
}

func TestRunFunctionSchedulesResume(t *testing.T) {
	server := setupFunctionTest(t)
	path := "/functions/" + functionIDTest + "/schedules/resume"
	server.AddResponse(path, 200, schedulesListJSON)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runFunctionSchedulesResume(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if reqs := server.Requests(path); len(reqs) != 1 || reqs[0].Method != "POST" {
		t.Fatalf("requests = %+v, want one POST", reqs)
	}
	if !strings.Contains(stdout, "enabled") {
		t.Errorf("expected the resumed state in the output, got %q", stdout)
	}
}

func TestRunFunctionSchedulesListEmpty(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/schedules", 200, `{"schedules":[],"state":null,"paused_reason":null}`)
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runFunctionSchedulesList(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No schedules found.") {
		t.Errorf("expected the empty message, got %q", stdout)
	}
}

func TestParseVariablesRejectsBadInput(t *testing.T) {
	for _, c := range []struct {
		json  string
		pairs []string
		want  string
	}{
		{`[1]`, nil, "failed to parse --vars JSON"},
		{`null`, nil, "--vars must be a JSON object"},
		{"", []string{"novalue"}, "expected key=value"},
	} {
		if _, err := parseVariables(c.json, c.pairs); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("parseVariables(%q, %v) = %v, want %q", c.json, c.pairs, err, c.want)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

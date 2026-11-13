package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/api"
)

func observation(t *testing.T, raw string) *api.Observation {
	t.Helper()
	var obs api.Observation
	if err := json.Unmarshal([]byte(raw), &obs); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return &obs
}

func merged(t *testing.T, description, states string) string {
	t.Helper()
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(states), &raw); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return mergeElementStates(description, decodeElementStates(&raw))
}

func TestMergeElementStatesWritesLiveStateIntoTags(t *testing.T) {
	description := strings.Join([]string{
		`_[:]Checkboxes`,
		`B1[:]<input type="checkbox"></input>`,
		`B2[:]<input type="checkbox"></input>`,
		`I1[:]<input type="email" name="email"></input>`,
		`I2[:]<select name="country"></select>`,
		`I3[:]<input type="password" name="pw"></input>`,
		`B3[:]<button aria_expanded="false">Menu</button>`,
		`B4[:]<button>Bold</button>`,
		`B5[:]<input type="checkbox"></input>`,
		`I4[:]<textarea name="note"></textarea>`,
	}, "\n")
	states := `{
		"B1": {"checked": false},
		"B2": {"checked": true},
		"I1": {"value": "a@b.co", "invalid": true, "error": "Enter a \"real\" email"},
		"I2": {"value": "fr", "options": [{"value": "de", "label": "Germany"}, {"value": "fr", "label": "France", "selected": true}]},
		"I3": {"filled": true},
		"B3": {"expanded": true},
		"B4": {"pressed": "mixed"},
		"B5": {"checked": true, "mixed": true},
		"I4": {"value": "line one\nline two", "valueTruncated": true},
		"X9": {"checked": true}
	}`
	want := strings.Join([]string{
		`_[:]Checkboxes`,
		`B1[:]<input type="checkbox"></input>`,
		`B2[:]<input type="checkbox" checked></input>`,
		`I1[:]<input type="email" name="email" invalid value="a@b.co" error="Enter a &quot;real&quot; email"></input>`,
		`I2[:]<select name="country" value="France"></select>`,
		`I3[:]<input type="password" name="pw" filled></input>`,
		`B3[:]<button aria_expanded="false">Menu</button>`,
		`B4[:]<button pressed="mixed">Bold</button>`,
		`B5[:]<input type="checkbox" mixed></input>`,
		`I4[:]<textarea name="note" value="line one&#10;line two..."></textarea>`,
	}, "\n")
	if got := merged(t, description, states); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeElementStatesDoesNotRepeatStateTheDescriptionShows(t *testing.T) {
	description := strings.Join([]string{
		`B2[:]<input type="checkbox" checked></input>`,
		`I1[:]<input type="text" name="q" value="shoes"></input>`,
		`I2[:]<input type="text" name="long" value="aaaaaaaaaa..."></input>`,
		`B3[:]<button selected>Tab "a>b"</button>`,
	}, "\n")
	states := `{
		"B2": {"checked": true},
		"I1": {"value": "shoes"},
		"I2": {"value": "aaaaaaaaaaaaaaaaaaaa"},
		"B3": {"selected": true}
	}`
	if got := merged(t, description, states); got != description {
		t.Errorf("got:\n%s\nwant it unchanged:\n%s", got, description)
	}
}

func TestMergeElementStatesReplacesTheServedValue(t *testing.T) {
	got := merged(t, `I1[:]<input type="text" name="q" value="served"></input>`, `{"I1": {"value": "typed since"}}`)
	want := `I1[:]<input type="text" name="q" value="typed since"></input>`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestMergeElementStatesLeavesCheckboxValuesAndMultilineTextAlone(t *testing.T) {
	description := "B1[:]<input type=\"checkbox\" value=\"on\"></input>\nB2[:]<a href=\"/x\">Two\nlines</a>"
	got := merged(t, description, `{"B1": {"checked": true, "value": "on"}, "B2": {"value": "ignored"}}`)
	want := "B1[:]<input type=\"checkbox\" value=\"on\" checked></input>\nB2[:]<a href=\"/x\">Two\nlines</a>"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderObservationHeaderTabsAndViewport(t *testing.T) {
	obs := observation(t, `{
		"metadata": {
			"title": "The Internet", "url": "https://the-internet.herokuapp.com/checkboxes",
			"tabs": [
				{"tab_id": 0, "title": "The Internet", "url": "https://the-internet.herokuapp.com/checkboxes"},
				{"tab_id": 1, "title": "Docs", "url": "https://docs.example.com"}
			],
			"viewport": {"scroll_x": 0, "scroll_y": 900, "viewport_width": 1280, "viewport_height": 900, "total_width": 1280, "total_height": 3000}
		},
		"screenshot": {}, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T10:00:01Z",
		"space": {"description": "B2[:]<input type=\"checkbox\"></input>", "interaction_actions": []},
		"element_states": {"B2": {"checked": true}}
	}`)
	want := strings.Join([]string{
		`The Internet - https://the-internet.herokuapp.com/checkboxes`,
		`Tabs (2):`,
		`  * 0 The Internet - https://the-internet.herokuapp.com/checkboxes`,
		`    1 Docs - https://docs.example.com`,
		`Viewport: 900-1800px of 3000px`,
		``,
		`B2[:]<input type="checkbox" checked></input>`,
	}, "\n")
	if got := renderObservation(obs); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderObservationWithoutStatesOrScroll(t *testing.T) {
	obs := observation(t, `{
		"metadata": {"title": "", "url": "https://example.com", "tabs": [{"tab_id": 0, "title": "", "url": "https://example.com"}],
			"viewport": {"scroll_x": 0, "scroll_y": 0, "viewport_width": 1280, "viewport_height": 900, "total_width": 1280, "total_height": 600}},
		"screenshot": {}, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T10:00:01Z",
		"space": {"description": "L1[:]<a href=\"/more\">More</a>", "interaction_actions": []}
	}`)
	want := "https://example.com\n\nL1[:]<a href=\"/more\">More</a>"
	if got := renderObservation(obs); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeElementStatesClearsStateUndoneSince(t *testing.T) {
	description := strings.Join([]string{
		`B1[:]<input type="checkbox" checked></input>`,
		`I1[:]<input type="text" name="q" value="served"></input>`,
		`B2[:]<button pressed>Bold</button>`,
		`I2[:]<input type="email" invalid></input>`,
		`B3[:]<input type="checkbox" value="on" checked></input>`,
		`I3[:]<input type="text" name="untouched" value="served"></input>`,
	}, "\n")
	states := `{
		"B1": {"checked": false},
		"I1": {"value": ""},
		"B2": {"pressed": false},
		"I2": {"invalid": false},
		"B3": {"checked": false, "value": "on"},
		"I3": {"checked": null}
	}`
	want := strings.Join([]string{
		`B1[:]<input type="checkbox"></input>`,
		`I1[:]<input type="text" name="q"></input>`,
		`B2[:]<button>Bold</button>`,
		`I2[:]<input type="email"></input>`,
		`B3[:]<input type="checkbox" value="on"></input>`,
		`I3[:]<input type="text" name="untouched" value="served"></input>`,
	}, "\n")
	if got := merged(t, description, states); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderObservationMarksNoTabWhenTheURLIsAmbiguous(t *testing.T) {
	obs := observation(t, `{
		"metadata": {"title": "Home", "url": "https://example.com",
			"tabs": [{"tab_id": 0, "title": "Home", "url": "https://example.com"}, {"tab_id": 1, "title": "Home", "url": "https://example.com"}],
			"viewport": {"scroll_x": 0, "scroll_y": 0, "viewport_width": 1280, "viewport_height": 900, "total_width": 1280, "total_height": 900}},
		"screenshot": {}, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T10:00:01Z",
		"space": {"description": "", "interaction_actions": []}
	}`)
	want := "Home - https://example.com\nTabs (2):\n    0 Home - https://example.com\n    1 Home - https://example.com"
	if got := renderObservation(obs); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestMergeElementStatesMatchesTheAPIsOwnRendering(t *testing.T) {
	// what the API writes itself for a control with an id: escaped, and cut at 60 characters
	long := strings.Repeat("x", 70)
	description := strings.Join([]string{
		`I1[:]<textarea name="note" value="say &quot;hi&quot; &lt;b&gt; &amp;&#10;bye"></textarea>`,
		`I2[:]<input type="text" name="long" value="` + strings.Repeat("x", 60) + `..."></input>`,
	}, "\n")
	states := `{"I1": {"value": "say \"hi\" <b> &\nbye"}, "I2": {"value": "` + long + `"}}`
	if got := merged(t, description, states); got != description {
		t.Errorf("got:\n%s\nwant it unchanged:\n%s", got, description)
	}
}

func TestMergeElementStatesCutsALongValueLikeTheAPI(t *testing.T) {
	got := merged(t, `I1[:]<input type="text" name="q"></input>`, `{"I1": {"value": "`+strings.Repeat("y", 61)+`"}}`)
	want := `I1[:]<input type="text" name="q" value="` + strings.Repeat("y", 60) + `..."></input>`
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

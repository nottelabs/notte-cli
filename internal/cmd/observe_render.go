package cmd

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/nottelabs/notte-cli/internal/api"
)

// An observation's description lists the page's interactive elements, one per
// line, as `<id>[:]<tag attributes>text</tag>`. The HTML attributes it shows are
// the ones the page was served with: a box ticked since, a value typed since
// or the option chosen in a select are only in the observation's
// element_states. renderObservation writes that live state into each
// element's tag, the way the API itself writes it for controls without an id:
//
//	B2[:]<input type="checkbox" checked></input>
//	I1[:]<input type="email" name="email" value="a@b.co"></input>
//	I2[:]<select name="country" value="France"></select>
//	I3[:]<input type="password" filled></input>
//	B4[:]<button mixed>Bold</button>
//
// State that the description already shows is not written twice.

// stateFlags are the boolean states written as bare attributes, in this order.
var stateFlags = []string{"checked", "mixed", "pressed", "selected", "expanded", "filled", "invalid"}

// elementLine matches the first line of an element with an id: its id and the
// start of its tag.
var elementLine = regexp.MustCompile(`^([A-Za-z]+[0-9]+)\[:\]<([A-Za-z][A-Za-z0-9-]*)`)

// renderObservation is the text form of `notte page observe`: the page's title
// and URL, the other open tabs, where the viewport is on a page taller than
// it, and the description with every element's live state.
func renderObservation(obs *api.Observation) string {
	var b strings.Builder
	meta := obs.Metadata
	header := strings.TrimSpace(meta.Title)
	if meta.Url != "" {
		if header != "" {
			header += " - "
		}
		header += meta.Url
	}
	if header != "" {
		b.WriteString(header + "\n")
	}
	if len(meta.Tabs) > 1 {
		fmt.Fprintf(&b, "Tabs (%d):\n", len(meta.Tabs))
		// the metadata does not name the current tab: mark it only when one tab has the page's URL
		matching := 0
		for _, tab := range meta.Tabs {
			if tab.Url == meta.Url {
				matching++
			}
		}
		for _, tab := range meta.Tabs {
			marker := " "
			if matching == 1 && tab.Url == meta.Url {
				marker = "*"
			}
			fmt.Fprintf(&b, "  %s %d %s - %s\n", marker, tab.TabId, strings.TrimSpace(tab.Title), tab.Url)
		}
	}
	if line := viewportLine(meta.Viewport); line != "" {
		b.WriteString(line + "\n")
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(mergeElementStates(obs.Space.Description, decodeElementStates(obs.ElementStates)))
	return strings.TrimRight(b.String(), "\n")
}

// viewportLine says which part of a page taller than the viewport is shown,
// so that the content above and below it (listed without ids) can be scrolled
// to. It is empty for a page that fits.
func viewportLine(v api.ViewportData) string {
	if v.ViewportHeight <= 0 || v.TotalHeight <= v.ViewportHeight {
		return ""
	}
	bottom := v.ScrollY + v.ViewportHeight
	if bottom > v.TotalHeight {
		bottom = v.TotalHeight
	}
	return fmt.Sprintf("Viewport: %d-%dpx of %dpx", v.ScrollY, bottom, v.TotalHeight)
}

// decodeElementStates reads the observation's element_states (typed loosely
// by the generated client) into ElementState values, by element id.
func decodeElementStates(raw *map[string]interface{}) map[string]api.ElementState {
	if raw == nil || len(*raw) == 0 {
		return nil
	}
	states := make(map[string]api.ElementState, len(*raw))
	for id, value := range *raw {
		data, err := json.Marshal(value)
		if err != nil {
			continue
		}
		var state api.ElementState
		if json.Unmarshal(data, &state) == nil {
			states[id] = state
		}
	}
	return states
}

// mergeElementStates writes each element's live state into its tag in the
// description. Elements without a state, and states of elements the
// description does not list, are left as they are.
func mergeElementStates(description string, states map[string]api.ElementState) string {
	if len(states) == 0 {
		return description
	}
	lines := strings.Split(description, "\n")
	for i, line := range lines {
		m := elementLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		state, ok := states[m[1]]
		if !ok {
			continue
		}
		lines[i] = withState(line, len(m[0]), strings.ToLower(m[2]), state)
	}
	return strings.Join(lines, "\n")
}

// withState rewrites the opening tag of one element line. tagStart is where
// its attributes start (just after the tag name).
func withState(line string, tagStart int, tag string, state api.ElementState) string {
	tagEnd := openingTagEnd(line, tagStart)
	if tagEnd < 0 {
		// an attribute value the scan cannot follow: leave the line as the API wrote it
		return line
	}
	attrs := line[tagStart:tagEnd]
	present := func(name string) bool { return hasAttribute(attrs, name) }
	inputType := strings.ToLower(attributeValue(attrs, "type"))

	var add []string
	for _, flag := range stateFlags {
		// on: the element is in this state now; known: the state says so either way
		on, known := false, false
		switch flag {
		case "checked":
			on, known = isTrue(state.Checked) && !isTrue(state.Mixed), state.Checked != nil
		case "mixed":
			// a partly checked box (aria-checked="mixed") or a partly pressed toggle (aria-pressed="mixed"), as the API writes both
			pressedMixed := pressedState(state.Pressed) == "mixed"
			on, known = isTrue(state.Mixed) || pressedMixed, state.Mixed != nil || pressedMixed
		case "pressed":
			pressed := pressedState(state.Pressed)
			on, known = pressed == "true", pressed != ""
		case "selected":
			on, known = isTrue(state.Selected), state.Selected != nil
		case "expanded":
			// aria_expanded is shown as an attribute already when the page sets it
			on, known = isTrue(state.Expanded) && !present("aria_expanded"), state.Expanded != nil
		case "filled":
			// a password's value is never sent: say that it holds one
			on, known = isTrue(state.Filled) && state.Value == nil, state.Filled != nil
		case "invalid":
			on, known = isTrue(state.Invalid), state.Invalid != nil
		}
		switch {
		case on && !present(flag):
			add = append(add, flag)
		case !on && known && present(flag):
			// unchecked, released or cleared since
			attrs = removeAttribute(attrs, flag)
		}
	}
	if s := deref(state.Sort); s != "" && !present("sort") && !present("aria_sort") {
		add = append(add, fmt.Sprintf("sort=%q", s))
	}

	value, known := liveValue(tag, inputType, state)
	switch {
	case !known:
	case value == "":
		// emptied since it was served
		attrs = removeAttribute(attrs, "value")
	case !present("value"):
		add = append(add, "value="+quote(cutValue(value, isTrue(state.ValueTruncated))))
	case !sameValue(attributeValue(attrs, "value"), value):
		// the served value, not what the field holds now
		attrs = replaceAttribute(attrs, "value", quote(cutValue(value, isTrue(state.ValueTruncated))))
	}
	if msg := deref(state.Error); msg != "" && !present("error") {
		if isTrue(state.ErrorTruncated) {
			msg += "..."
		}
		add = append(add, "error="+quote(msg))
	}

	if len(add) > 0 {
		attrs += " " + strings.Join(add, " ")
	}
	return line[:tagStart] + attrs + line[tagEnd:]
}

// liveValue is what a field holds now: the text of a text field, or the
// labels of a select's chosen options. known is false when the state does not
// say: checkboxes, radios and buttons have no value worth showing (their
// `value` is what the form submits). An empty value is a known, empty field.
func liveValue(tag, inputType string, state api.ElementState) (string, bool) {
	if tag == "select" || state.Options != nil {
		var labels []string
		if state.Options != nil {
			for _, option := range *state.Options {
				if isTrue(option.Selected) {
					labels = append(labels, option.Label)
				}
			}
		}
		if len(labels) > 0 {
			return strings.Join(labels, ", "), true
		}
		if v := deref(state.Value); v != "" {
			return v, true
		}
		return "", false
	}
	switch inputType {
	case "checkbox", "radio", "button", "submit", "reset", "image", "file", "hidden":
		return "", false
	}
	if tag == "button" || tag == "a" {
		return "", false
	}
	if state.Value == nil {
		return "", false
	}
	return *state.Value, true
}

// sameValue says whether a value attribute already shows the live value. The
// description cuts attributes, ending them with "...".
func sameValue(shown, live string) bool {
	if shown == live {
		return true
	}
	if cut, ok := strings.CutSuffix(shown, "..."); ok && cut != "" {
		return strings.HasPrefix(live, cut)
	}
	return false
}

func pressedState(p *api.ElementState_Pressed) string {
	if p == nil {
		return ""
	}
	data, err := p.MarshalJSON()
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(string(data)) {
	case "true":
		return "true"
	case "false":
		return "false"
	case `"mixed"`:
		return "mixed"
	}
	return ""
}

func isTrue(b *bool) bool { return b != nil && *b }

// maxValueLen is where a rendered value is cut, as the API cuts attributes.
const maxValueLen = 60

// cutValue cuts a long value the way the API cuts attributes, ending it with
// "...". browserCut says the browser already cut it (ElementState.valueTruncated).
func cutValue(v string, browserCut bool) string {
	if runes := []rune(v); len(runes) > maxValueLen {
		return string(runes[:maxValueLen]) + "..."
	}
	if browserCut {
		return v + "..."
	}
	return v
}

// attributeEscaper escapes a value the way the API does, so that it stays one
// attribute on one line.
var attributeEscaper = strings.NewReplacer(
	"&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;", "\n", "&#10;", "\r", "&#13;",
)

// quote writes an attribute value on one line, in double quotes.
func quote(s string) string {
	return `"` + attributeEscaper.Replace(s) + `"`
}

// openingTagEnd is the index of the ">" that closes the opening tag started
// before `from`, skipping quoted attribute values; -1 when there is none on
// this line.
func openingTagEnd(line string, from int) int {
	inQuote := false
	for i := from; i < len(line); i++ {
		switch line[i] {
		case '"':
			inQuote = !inQuote
		case '>':
			if !inQuote {
				return i
			}
		}
	}
	return -1
}

// attributeTokens splits the attributes of an opening tag into `name` and
// `name="value"` tokens.
func attributeTokens(attrs string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	for _, r := range attrs {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ' ' && !inQuote:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func attributeName(token string) string {
	name, _, _ := strings.Cut(token, "=")
	return strings.ToLower(name)
}

func hasAttribute(attrs, name string) bool {
	for _, token := range attributeTokens(attrs) {
		if attributeName(token) == name {
			return true
		}
	}
	return false
}

func attributeValue(attrs, name string) string {
	for _, token := range attributeTokens(attrs) {
		if attributeName(token) == name {
			_, value, _ := strings.Cut(token, "=")
			return html.UnescapeString(strings.Trim(value, `"`))
		}
	}
	return ""
}

func removeAttribute(attrs, name string) string {
	var kept []string
	for _, token := range attributeTokens(attrs) {
		if attributeName(token) != name {
			kept = append(kept, token)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return " " + strings.Join(kept, " ")
}

func replaceAttribute(attrs, name, quoted string) string {
	tokens := attributeTokens(attrs)
	for i, token := range tokens {
		if attributeName(token) == name {
			tokens[i] = name + "=" + quoted
		}
	}
	if len(tokens) == 0 {
		return attrs
	}
	return " " + strings.Join(tokens, " ")
}

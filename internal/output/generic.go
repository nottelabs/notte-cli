package output

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Print renders any value as text. Commands with a designed output print it
// themselves; this is what every other response falls back to, so it has to
// read well for data it has never seen: fields get plain-language labels,
// identifying fields come first, times are relative, and empty fields are
// left out.
func (f *TextFormatter) Print(data any) error {
	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	var text string
	switch v.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Slice, reflect.Array:
		blocks := make([]string, 0, v.Len())
		for i := range v.Len() {
			if value, nested := f.value(v.Index(i), ""); nested != "" {
				blocks = append(blocks, nested)
			} else if value != "" {
				blocks = append(blocks, value+"\n")
			}
		}
		separator := ""
		if v.Len() > 0 && isBlock(v.Index(0)) {
			separator = "\n"
		}
		text = strings.Join(blocks, separator)
	default:
		value, nested := f.value(v, "")
		if text = nested; nested == "" && value != "" {
			text = value + "\n"
		}
	}
	_, err := io.WriteString(f.Writer, text)
	return err
}

// now is replaced in tests.
var now = time.Now

// RelativeTime describes a recent moment as "2 min ago" and an older one by
// its date, in the local time zone.
func RelativeTime(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	switch elapsed := now.Sub(t); {
	case elapsed < 0:
		return t.In(now.Location()).Format("Jan 2 2006 15:04")
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%d min ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(elapsed.Hours()))
	case t.Year() == now.Year():
		return t.In(now.Location()).Format("Jan 2 15:04")
	}
	return t.In(now.Location()).Format("Jan 2 2006")
}

type row struct {
	label  string
	value  string // one-line value, or
	nested string // an indented block printed under the label
	rank   int
}

// rows prints labelled values with the values aligned in a column.
func (f *TextFormatter) rows(rows []row, indent string) string {
	width := 0
	for _, r := range rows {
		if r.nested == "" && len(r.label) > width {
			width = len(r.label)
		}
	}
	var b strings.Builder
	for _, r := range rows {
		if r.nested != "" {
			b.WriteString(indent + f.Label(r.label) + "\n" + r.nested)
			continue
		}
		b.WriteString(indent + f.Label(r.label) + strings.Repeat(" ", width-len(r.label)+3) + r.value + "\n")
	}
	return b.String()
}

// value renders v either as a one-line value or as a nested block. Both are
// empty for a value with nothing to show.
func (f *TextFormatter) value(v reflect.Value, indent string) (string, string) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return "", ""
		}
		v = v.Elem()
	}
	if t, ok := timeOf(v); ok {
		if t.IsZero() {
			return "", ""
		}
		return RelativeTime(t, now()), ""
	}
	switch v.Kind() {
	case reflect.Invalid:
		return "", ""
	case reflect.String:
		return v.String(), ""
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, v.Type().Bits()), ""
	case reflect.Struct:
		if s, ok := v.Interface().(fmt.Stringer); ok {
			return s.String(), ""
		}
		return "", f.structBlock(v, indent)
	case reflect.Map:
		return f.mapValue(v, indent)
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return "", ""
		}
		if !isBlock(v.Index(0)) {
			items := make([]string, 0, v.Len())
			for i := range v.Len() {
				item, _ := f.value(v.Index(i), indent)
				items = append(items, item)
			}
			return strings.Join(items, ", "), ""
		}
		blocks := make([]string, 0, v.Len())
		for i := range v.Len() {
			if _, nested := f.value(v.Index(i), indent); nested != "" {
				blocks = append(blocks, nested)
			}
		}
		return "", strings.Join(blocks, "\n")
	}
	return fmt.Sprintf("%v", v.Interface()), ""
}

// structBlock renders a struct's exported fields, most identifying first.
func (f *TextFormatter) structBlock(v reflect.Value, indent string) string {
	var out []row
	for i := range v.NumField() {
		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		fv := v.Field(i)
		value, nested := f.value(fv, indent+"  ")
		if value == "" && nested == "" {
			continue
		}
		_, isTime := timeOf(indirect(fv))
		out = append(out, row{label: Humanize(field.Name, isTime), value: value, nested: nested, rank: fieldRank(field.Name, isTime)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].rank < out[j].rank })
	return f.rows(out, indent)
}

// mapValue renders a map with its keys sorted, so the same data always prints
// the same way.
func (f *TextFormatter) mapValue(v reflect.Value, indent string) (string, string) {
	if v.Len() == 0 {
		return "", ""
	}
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
	var out []row
	for _, key := range keys {
		value, nested := f.value(v.MapIndex(key), indent+"  ")
		if value == "" && nested == "" {
			continue
		}
		out = append(out, row{label: fmt.Sprint(key.Interface()), value: value, nested: nested})
	}
	return "", f.rows(out, indent)
}

func indirect(v reflect.Value) reflect.Value {
	for (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && !v.IsNil() {
		v = v.Elem()
	}
	return v
}

// isBlock reports whether v prints as several lines.
func isBlock(v reflect.Value) bool {
	v = indirect(v)
	if _, ok := timeOf(v); ok {
		return false
	}
	switch v.Kind() {
	case reflect.Struct:
		_, stringer := v.Interface().(fmt.Stringer)
		return !stringer
	case reflect.Map:
		return true
	}
	return false
}

// timeOf finds the time in a time.Time or in a struct embedding one, which is
// how the API client types its timestamps.
func timeOf(v reflect.Value) (time.Time, bool) {
	if v.Kind() != reflect.Struct || !v.CanInterface() {
		return time.Time{}, false
	}
	if t, ok := v.Interface().(time.Time); ok {
		return t, true
	}
	if field := v.FieldByName("Time"); field.IsValid() && field.CanInterface() && v.NumField() == 1 {
		if t, ok := field.Interface().(time.Time); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// fieldRank orders fields: what the thing is called, its IDs, its state, the
// rest, and timestamps last.
func fieldRank(name string, isTime bool) int {
	switch {
	case name == "Name":
		return 0
	case strings.HasSuffix(name, "Id") || name == "ID":
		return 1
	case name == "Status":
		return 2
	case name == "Description":
		return 3
	case isTime:
		return 9
	}
	return 5
}

var acronyms = map[string]string{
	"id": "ID", "url": "URL", "cdp": "CDP", "api": "API", "llm": "LLM", "ip": "IP", "sms": "SMS",
	"js": "JS", "json": "JSON", "html": "HTML", "mfa": "MFA", "otp": "OTP", "http": "HTTP", "cpu": "CPU",
}

// Humanize turns a field name into a label: "FunctionRunId" becomes
// "Function run ID". A timestamp drops its "At": "CreatedAt" is "Created".
func Humanize(name string, isTime bool) string {
	var words []string
	start := 0
	runes := []rune(name)
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || (unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i-1])) || runes[i] == '_' {
			if word := strings.Trim(string(runes[start:i]), "_"); word != "" {
				words = append(words, strings.ToLower(word))
			}
			start = i
		}
	}
	if isTime && len(words) > 1 && words[len(words)-1] == "at" {
		words = words[:len(words)-1]
	}
	for i, word := range words {
		if upper, ok := acronyms[word]; ok {
			words[i] = upper
		} else if i == 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

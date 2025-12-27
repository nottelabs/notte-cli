package output

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/muesli/termenv"
)

// TextFormatter outputs human-readable text
type TextFormatter struct {
	Writer  io.Writer
	NoColor bool
}

var output = termenv.NewOutput(os.Stdout)

func (f *TextFormatter) Print(data any) error {
	v := reflect.ValueOf(data)

	// Handle maps as key-value display
	if v.Kind() == reflect.Map {
		return f.printKeyValue(data)
	}

	// Handle slices as tables
	if v.Kind() == reflect.Slice {
		return f.printSlice(data)
	}

	// Handle structs as key-value
	if v.Kind() == reflect.Struct {
		return f.printStruct(data)
	}

	// Default: just print
	_, err := fmt.Fprintln(f.Writer, data)
	return err
}

func (f *TextFormatter) printKeyValue(data any) error {
	v := reflect.ValueOf(data)
	w := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)
		label := f.colorize(fmt.Sprintf("%v:", key.Interface()), termenv.ANSICyan)
		_, _ = fmt.Fprintf(w, "%s\t%v\n", label, val.Interface())
	}

	return w.Flush()
}

func (f *TextFormatter) printStruct(data any) error {
	v := reflect.ValueOf(data)
	t := v.Type()
	w := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		label := f.colorize(field.Name+":", termenv.ANSICyan)
		_, _ = fmt.Fprintf(w, "%s\t%v\n", label, v.Field(i).Interface())
	}

	return w.Flush()
}

func (f *TextFormatter) printSlice(data any) error {
	// For slices, print each item
	v := reflect.ValueOf(data)
	for i := 0; i < v.Len(); i++ {
		if err := f.Print(v.Index(i).Interface()); err != nil {
			return err
		}
		if i < v.Len()-1 {
			_, _ = fmt.Fprintln(f.Writer)
		}
	}
	return nil
}

// PrintTable prints data as a table with headers
func (f *TextFormatter) PrintTable(headers []string, data []map[string]any) error {
	w := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	// Print headers
	coloredHeaders := make([]string, len(headers))
	for i, h := range headers {
		coloredHeaders[i] = f.colorize(h, termenv.ANSICyan)
	}
	_, _ = fmt.Fprintln(w, strings.Join(coloredHeaders, "\t"))

	// Print rows
	for _, row := range data {
		values := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := row[h]; ok {
				values[i] = fmt.Sprintf("%v", v)
			}
		}
		_, _ = fmt.Fprintln(w, strings.Join(values, "\t"))
	}

	return w.Flush()
}

func (f *TextFormatter) PrintError(err error) {
	errText := f.colorize("Error:", termenv.ANSIRed)
	fmt.Fprintf(os.Stderr, "%s %s\n", errText, err.Error())
}

func (f *TextFormatter) colorize(s string, color termenv.ANSIColor) string {
	if f.NoColor {
		return s
	}
	return output.String(s).Foreground(color).String()
}

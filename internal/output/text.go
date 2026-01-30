package output

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/muesli/termenv"

	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

// TextFormatter outputs human-readable text
type TextFormatter struct {
	Writer  io.Writer
	NoColor bool
}

var output = termenv.NewOutput(os.Stdout)

func (f *TextFormatter) Print(data any) error {
	v := reflect.ValueOf(data)

	// Handle pointers by dereferencing
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			_, err := fmt.Fprintln(f.Writer, "<nil>")
			return err
		}
		v = v.Elem()
	}

	// Handle maps as key-value display
	if v.Kind() == reflect.Map {
		return f.printKeyValue(v.Interface())
	}

	// Handle slices as tables
	if v.Kind() == reflect.Slice {
		return f.printSlice(v.Interface())
	}

	// Handle structs as key-value
	if v.Kind() == reflect.Struct {
		return f.printStruct(v.Interface())
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

		fieldValue := v.Field(i)

		// Skip nil pointers, nil slices, nil maps, and nil interfaces
		switch fieldValue.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
			if fieldValue.IsNil() {
				continue
			}
		}

		label := f.colorize(field.Name+":", termenv.ANSICyan)

		var displayValue any

		// Handle pointer fields by dereferencing
		if fieldValue.Kind() == reflect.Ptr {
			displayValue = fieldValue.Elem().Interface()
		} else {
			displayValue = fieldValue.Interface()
		}

		_, _ = fmt.Fprintf(w, "%s\t%v\n", label, displayValue)
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
	// For API errors, display "Error <status>: <message>"
	if apiErr, ok := err.(*apierrors.APIError); ok && apiErr.Message != "" {
		errText := f.colorize(fmt.Sprintf("Error %d:", apiErr.StatusCode), termenv.ANSIRed)
		fmt.Fprintf(os.Stderr, "%s %s\n", errText, apiErr.Message)
		return
	}
	errText := f.colorize("Error:", termenv.ANSIRed)
	fmt.Fprintf(os.Stderr, "%s %s\n", errText, err.Error())
}

func (f *TextFormatter) colorize(s string, color termenv.ANSIColor) string {
	if f.NoColor {
		return s
	}
	return output.String(s).Foreground(color).String()
}

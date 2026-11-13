package api

import "encoding/json"

// UnmarshalJSON accepts both FastAPI 422 shapes: a list of validation errors
// and a plain string detail. The generated type only models the list, so a
// string detail used to fail decoding and hide the server's message behind a
// Go unmarshal error. A string becomes a single ValidationError with that Msg.
func (e *HTTPValidationError) UnmarshalJSON(data []byte) error {
	var raw struct {
		Detail json.RawMessage `json:"detail"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Detail = nil
	if len(raw.Detail) == 0 || string(raw.Detail) == "null" {
		return nil
	}
	var list []ValidationError
	if err := json.Unmarshal(raw.Detail, &list); err == nil {
		e.Detail = &list
		return nil
	}
	var msg string
	if err := json.Unmarshal(raw.Detail, &msg); err != nil {
		// Unknown shape: keep decoding going so the caller can still report
		// the raw response body instead of a decode error.
		msg = string(raw.Detail)
	}
	e.Detail = &[]ValidationError{{Msg: msg}}
	return nil
}

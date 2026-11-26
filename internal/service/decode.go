package service

import (
	"encoding/json"
	"errors"
	"example.local/sector-drift/internal/domain"
	"io"
	"mime"
	"net/http"
)

func decode(w http.ResponseWriter, r *http.Request) (domain.Request, int, error) {
	var input domain.Request
	status, err := decodeInto(w, r, &input, false)
	return input, status, err
}

// decodeInto strictly decodes a single JSON document into dst. When allowEmpty
// is true, an empty body yields the zero value.
func decodeInto(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) (int, error) {
	if r.Body != nil {
		defer r.Body.Close()
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	hasBody := r.ContentLength != 0
	if err != nil || media != "application/json" {
		// An empty body is tolerated for endpoints that take path/query params.
		if allowEmpty && (!hasBody || r.ContentLength == 0) {
			return 0, nil
		}
		return 415, errors.New("expected application/json")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 3<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return 0, nil
		}
		return decodeStatus(err), errors.New("invalid request document")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return decodeStatus(err), errors.New("expected exactly one JSON object")
	}
	return 0, nil
}

func decodeStatus(err error) int {
	var size *http.MaxBytesError
	if errors.As(err, &size) {
		return 413
	}
	return 400
}

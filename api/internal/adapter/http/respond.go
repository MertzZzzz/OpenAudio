// Package http exposes the use cases over a JSON REST API (chi router).
package http

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// Error responses follow components.schemas.Error in openapi.yaml:
// {"code", "message", "requestId", "issues": [{"severity","path","message"}]}.
type issueWithSeverity struct {
	Severity string `json:"severity"`
	domain.FieldError
}

var statusByKind = map[domain.Kind]int{
	domain.KindValidation:   http.StatusUnprocessableEntity,
	domain.KindNotFound:     http.StatusNotFound,
	domain.KindConflict:     http.StatusConflict,
	domain.KindUnauthorized: http.StatusUnauthorized,
	domain.KindForbidden:    http.StatusForbidden,
	domain.KindRateLimited:  http.StatusTooManyRequests,
	domain.KindInternal:     http.StatusInternalServerError,
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func (a *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var de *domain.Error
	if !errors.As(err, &de) {
		de = domain.Internal(err)
	}
	status := statusByKind[de.Kind]
	if status == 0 {
		status = http.StatusInternalServerError
	}
	if status >= 500 {
		a.log.ErrorContext(r.Context(), "request failed", "err", err, "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()))
	}
	body := map[string]any{"code": de.Code, "message": de.Message}
	if id := middleware.GetReqID(r.Context()); id != "" {
		body["requestId"] = id
	}
	if len(de.Fields) > 0 {
		issues := make([]issueWithSeverity, len(de.Fields))
		for i, f := range de.Fields {
			issues[i] = issueWithSeverity{Severity: "error", FieldError: f}
		}
		body["issues"] = issues
	}
	writeJSON(w, status, body)
}

const maxBodyBytes = 1 << 20

var errBadJSON = func(msg string) *domain.Error {
	return &domain.Error{Kind: domain.KindValidation, Code: "invalid_json", Message: msg}
}

// decodeJSON strictly decodes the request body. An empty body is allowed when optional is true.
func decodeJSON(r *http.Request, dst any, optional bool) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mt, _, err := mime.ParseMediaType(ct); err != nil || mt != "application/json" {
			return &domain.Error{Kind: domain.KindValidation, Code: "unsupported_media_type", Message: "Content-Type must be application/json"}
		}
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) && optional {
			return nil
		}
		if errors.Is(err, io.EOF) {
			return errBadJSON("request body is required")
		}
		return errBadJSON("malformed JSON: " + err.Error())
	}
	if dec.More() {
		return errBadJSON("request body must contain a single JSON object")
	}
	return nil
}

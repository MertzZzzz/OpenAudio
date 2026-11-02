package http

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/uploads"
)

func uploadID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "uploadId"))
	if err != nil {
		return uuid.Nil, domain.NotFound("upload_not_found", "upload not found")
	}
	return id, nil
}

func (a *API) createUpload(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Digest      string `json:"digest"`
		Size        int64  `json:"size"`
		ContentType string `json:"contentType"`
	}
	if err := decodeJSON(r, &in, false); err != nil {
		a.writeError(w, r, err)
		return
	}
	u, err := a.up.Create(r.Context(), actor(r), uploads.CreateInput{Digest: in.Digest, Size: in.Size, ContentType: in.ContentType})
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	status := http.StatusCreated
	if u.Status == domain.UploadDone {
		status = http.StatusOK // already stored: nothing to upload
	}
	writeJSON(w, status, toUploadDTO(u, nil))
}

func (a *API) getUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uploadID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	st, err := a.up.Get(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUploadDTO(st.Upload, st.Parts))
}

// putPart streams the raw request body (application/octet-stream) into one multipart part.
func (a *API) putPart(w http.ResponseWriter, r *http.Request) {
	id, err := uploadID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	n, err := strconv.Atoi(chi.URLParam(r, "partNumber"))
	if err != nil {
		a.writeError(w, r, domain.Validation(domain.FieldError{Field: "partNumber", Message: "must be an integer"}))
		return
	}
	if r.ContentLength < 0 {
		a.writeError(w, r, domain.Validation(domain.FieldError{Field: "Content-Length", Message: "Content-Length is required"}))
		return
	}
	body := http.MaxBytesReader(w, r.Body, r.ContentLength)
	p, err := a.up.PutPart(r.Context(), actor(r), id, n, body, r.ContentLength)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, uploadPartDTO{Number: p.Number, Size: p.Size})
}

func (a *API) completeUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uploadID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	u, err := a.up.Complete(r.Context(), actor(r), id)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toUploadDTO(u, nil))
}

func (a *API) abortUpload(w http.ResponseWriter, r *http.Request) {
	id, err := uploadID(r)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	if err := a.up.Abort(r.Context(), actor(r), id); err != nil {
		a.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

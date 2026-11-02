package http_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

// verifiedUser registers and confirms the email through the API, returning an access token.
func (r *rig) verifiedUser(t *testing.T, email string) string {
	t.Helper()
	r.register(t, email)
	tok := tokenRe.FindStringSubmatch(r.mail.Last().Body)[1]
	require.Equal(t, http.StatusNoContent, r.do(t, req{method: "POST", path: "/api/v1/auth/email/verify", body: map[string]string{"token": tok}}).Code)
	w := r.do(t, req{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": "a-good-password"}})
	return decode(t, w)["accessToken"].(string)
}

func (r *rig) uploadBlob(t *testing.T, token string, data []byte) {
	t.Helper()
	w := r.do(t, req{method: "POST", path: "/api/v1/uploads", bearer: token, body: map[string]any{"digest": sum(data), "size": len(data), "contentType": "audio/wav"}})
	require.Contains(t, []int{200, 201}, w.Code, w.Body.String())
	u := decode(t, w)
	id, parts, partSize := u["id"].(string), int(u["partCount"].(float64)), int(u["partSize"].(float64))
	for n := 1; n <= parts; n++ {
		end := n * partSize
		if end > len(data) {
			end = len(data)
		}
		w := r.do(t, req{method: "PUT", path: fmt.Sprintf("/api/v1/uploads/%s/parts/%d", id, n), bearer: token, body: data[(n-1)*partSize : end]})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	if parts > 0 {
		w = r.do(t, req{method: "POST", path: "/api/v1/uploads/" + id + "/complete", bearer: token})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "done", decode(t, w)["status"])
	}
}

func TestPublishingFlowOverHTTP(t *testing.T) {
	r := newRig(t, nil)
	owner := r.verifiedUser(t, "owner@acme.audio")

	// Publisher → package.
	w := r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: owner, body: map[string]string{"slug": "acme", "name": "Acme Audio"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "owner", decode(t, w)["role"])
	w = r.do(t, req{method: "GET", path: "/api/v1/me/publishers", bearer: owner})
	assert.Len(t, decodeList(t, w), 1)
	w = r.do(t, req{method: "GET", path: "/api/v1/publishers/acme"})
	assert.Equal(t, "Acme Audio", decode(t, w)["name"])

	w = r.do(t, req{method: "POST", path: "/api/v1/packages", bearer: owner, body: map[string]string{"publisher": "acme", "slug": "kit", "name": "Warm Kit", "kind": "sample-pack", "description": "Dusty drums"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	pkg := decode(t, w)
	assert.Equal(t, "acme.kit", pkg["id"])
	assert.Nil(t, pkg["latestVersion"])

	// Draft with an invalid manifest → field issues under manifest.*
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases", bearer: owner, body: map[string]any{"manifest": map[string]any{"version": "1.0.0"}}})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := decode(t, w)
	assert.Equal(t, "manifest_invalid", body["code"])
	assert.NotEmpty(t, body["issues"])

	kick := []byte("RIFF....WAVE kick sample bytes")
	mf := map[string]any{
		"version":   "1.0.0",
		"license":   map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy":  map[string]any{"genres": map[string]any{"primary": "lo-fi"}, "instruments": []string{"drums"}, "moods": []string{"warm"}},
		"musical":   map[string]any{"bpm": map[string]any{"value": 88}},
		"artifacts": []map[string]any{{"kind": "sample", "path": "one-shots/kick.wav", "digest": sum(kick), "size": len(kick), "contentType": "audio/wav"}},
		"tags":      map[string]any{"free": []string{"dusty"}},
	}
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases", bearer: owner, body: map[string]any{"changelog": "First", "manifest": mf}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	draft := decode(t, w)
	assert.Equal(t, "draft", draft["release"].(map[string]any)["status"])

	// Drafts are invisible to the public.
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/versions"})
	assert.Empty(t, decodeList(t, w))
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/releases/1.0.0"}).Code)

	// Publishing before the blob is uploaded fails with a precise issue.
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases/1.0.0/publish", bearer: owner})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body = decode(t, w)
	assert.Equal(t, "release_incomplete", body["code"])
	assert.Equal(t, "manifest.artifacts.0.digest", body["issues"].([]any)[0].(map[string]any)["path"])

	r.uploadBlob(t, owner, kick)
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases/1.0.0/publish", bearer: owner})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "published", decode(t, w)["status"])

	// Public reads.
	w = r.do(t, req{method: "GET", path: "/api/v1/packages?kind=sample-pack&q=warm"})
	list := decode(t, w)
	assert.EqualValues(t, 1, list["total"])
	item := list["items"].([]any)[0].(map[string]any)
	assert.Equal(t, "1.0.0", item["latestVersion"])
	assert.Equal(t, "lo-fi", item["taxonomy"].(map[string]any)["genres"].(map[string]any)["primary"])
	assert.Equal(t, []any{"dusty"}, item["tags"])
	assert.EqualValues(t, 1, item["stats"].(map[string]any)["samples"])

	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/versions/1.0.0/manifest"})
	require.Equal(t, http.StatusOK, w.Code)
	digest := w.Header().Get("X-Content-Digest")
	assert.Equal(t, digest, sum(w.Body.Bytes()), "served bytes match the digest")
	var served map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &served))
	assert.Equal(t, "acme.kit", served["id"])
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/versions/1.0.0/manifest", headers: map[string]string{"If-None-Match": `"` + digest + `"`}})
	assert.Equal(t, http.StatusNotModified, w.Code)

	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/releases/1.0.0"})
	rel := decode(t, w)
	arts := rel["artifacts"].([]any)
	require.Len(t, arts, 2)
	for _, a := range arts {
		assert.NotEmpty(t, a.(map[string]any)["downloadUrl"])
	}

	// Downloads redirect to presigned URLs and are counted (manifest downloads are not).
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/releases/1.0.0/files/one-shots/kick.wav"})
	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Location"), "https://blobs.test/blobs/")
	assert.Contains(t, w.Header().Get("Location"), "filename=kick.wav")
	assert.Equal(t, http.StatusFound, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/releases/1.0.0/files/manifest.json"}).Code)
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit/releases/1.0.0/files/nope.wav"}).Code)
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit"})
	assert.EqualValues(t, 1, decode(t, w)["downloads"])

	// Immutability over HTTP.
	w = r.do(t, req{method: "PUT", path: "/api/v1/packages/acme/kit/releases/1.0.0", bearer: owner, body: map[string]any{"changelog": "edit"}})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "release_immutable", decode(t, w)["code"])
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases", bearer: owner, body: map[string]any{"manifest": mf}})
	assert.Equal(t, "version_exists", decode(t, w)["code"])

	// Yank.
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases/1.0.0/yank", bearer: owner})
	require.Equal(t, http.StatusOK, w.Code)
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/kit"})
	assert.Nil(t, decode(t, w)["latestVersion"])

	// Dashboard view includes every release.
	w = r.do(t, req{method: "GET", path: "/api/v1/publishers/acme/packages", bearer: owner})
	rows := decodeList(t, w)
	require.Len(t, rows, 1)
	assert.Len(t, rows[0].(map[string]any)["releases"], 1)
}

func decodeList(t *testing.T, w interface{ Result() *http.Response }) []any {
	t.Helper()
	var out []any
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&out))
	return out
}

func TestPublishingPermissionsOverHTTP(t *testing.T) {
	r := newRig(t, nil)
	owner := r.verifiedUser(t, "owner@acme.audio")
	r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: owner, body: map[string]string{"slug": "acme", "name": "Acme Audio"}})
	r.do(t, req{method: "POST", path: "/api/v1/packages", bearer: owner, body: map[string]string{"publisher": "acme", "slug": "kit", "name": "Kit", "kind": "sample-pack"}})

	stranger, _ := r.register(t, "stranger@example.com")
	w := r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: stranger, body: map[string]string{"slug": "mine", "name": "Mine"}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "email_unverified", decode(t, w)["code"])

	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases", bearer: stranger, body: map[string]any{"manifest": map[string]any{}}})
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = r.do(t, req{method: "POST", path: "/api/v1/uploads", bearer: stranger, body: map[string]any{"digest": sum([]byte("x")), "size": 1}})
	assert.Equal(t, "not_a_publisher", decode(t, w)["code"])
	assert.Equal(t, http.StatusUnauthorized, r.do(t, req{method: "POST", path: "/api/v1/packages", body: map[string]string{}}).Code)

	// Owner adds the stranger as uploader → can upload and draft, still cannot publish.
	w = r.do(t, req{method: "POST", path: "/api/v1/publishers/acme/members", bearer: owner, body: map[string]string{"email": "stranger@example.com", "role": "uploader"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	userID := decode(t, w)["userId"].(string)
	w = r.do(t, req{method: "GET", path: "/api/v1/publishers/acme/members", bearer: stranger})
	assert.Len(t, decodeList(t, w), 2)

	data := []byte("snare!")
	r.uploadBlob(t, stranger, data)
	mf := map[string]any{"version": "0.1.0", "license": map[string]any{"content": "MIT", "commercialUse": true, "attributionRequired": true},
		"taxonomy":  map[string]any{"genres": map[string]any{"primary": "techno"}, "instruments": []string{}, "moods": []string{}},
		"artifacts": []map[string]any{{"kind": "sample", "path": "snare.wav", "digest": sum(data), "size": len(data)}}}
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases", bearer: stranger, body: map[string]any{"manifest": mf}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotEmpty(t, decode(t, w)["warnings"], "missing moods etc. are warnings")
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/kit/releases/0.1.0/publish", bearer: stranger})
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = r.do(t, req{method: "DELETE", path: "/api/v1/publishers/acme/members/" + userID, bearer: owner})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUploadEndpoints(t *testing.T) {
	r := newRig(t, nil)
	owner := r.verifiedUser(t, "owner@acme.audio")
	r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: owner, body: map[string]string{"slug": "acme", "name": "Acme Audio"}})

	claimed := []byte("honest!!")
	w := r.do(t, req{method: "POST", path: "/api/v1/uploads", bearer: owner, body: map[string]any{"digest": sum(claimed), "size": len(claimed)}})
	require.Equal(t, http.StatusCreated, w.Code)
	u := decode(t, w)
	id := u["id"].(string)
	assert.EqualValues(t, 2, u["partCount"])

	w = r.do(t, req{method: "PUT", path: "/api/v1/uploads/" + id + "/parts/1", bearer: owner, body: []byte("abc")})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "parts must have the exact size")
	r.do(t, req{method: "PUT", path: "/api/v1/uploads/" + id + "/parts/1", bearer: owner, body: []byte("EVIL")})
	r.do(t, req{method: "PUT", path: "/api/v1/uploads/" + id + "/parts/2", bearer: owner, body: []byte("!!!!")})
	w = r.do(t, req{method: "GET", path: "/api/v1/uploads/" + id, bearer: owner})
	assert.Len(t, decode(t, w)["parts"], 2, "progress is resumable")

	w = r.do(t, req{method: "POST", path: "/api/v1/uploads/" + id + "/complete", bearer: owner})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, "digest_mismatch", decode(t, w)["code"])
	assert.Empty(t, r.blobs.Keys(), "no blob, no temporary object")

	// Uploading content that already exists returns "done" immediately.
	r.uploadBlob(t, owner, claimed)
	w = r.do(t, req{method: "POST", path: "/api/v1/uploads", bearer: owner, body: map[string]any{"digest": sum(claimed), "size": len(claimed)}})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "done", decode(t, w)["status"])
}

func TestSchemaAndValidateEndpoints(t *testing.T) {
	r := newRig(t, nil)
	w := r.do(t, req{method: "GET", path: "/api/v1/schemas/openaudio.package/v1"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://openaudio.dev/schemas/openaudio.package/v1.json", decode(t, w)["$id"])

	w = r.do(t, req{method: "POST", path: "/api/v1/manifests/validate", body: map[string]any{"manifest": map[string]any{"schema": "openaudio.package/v1"}}})
	require.Equal(t, http.StatusOK, w.Code)
	body := decode(t, w)
	assert.Equal(t, false, body["valid"])
	assert.NotEmpty(t, body["issues"])
}

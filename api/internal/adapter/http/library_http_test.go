package http_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publish creates (if needed) and publishes acme/<slug>@<version> with optional dependencies.
func (r *rig) publish(t *testing.T, owner, slug, version string, deps map[string]string) {
	t.Helper()
	r.do(t, req{method: "POST", path: "/api/v1/packages", bearer: owner, body: map[string]string{"publisher": "acme", "slug": slug, "name": strings.ToUpper(slug[:1]) + slug[1:], "kind": "sample-pack"}})
	data := []byte(slug + "@" + version)
	r.uploadBlob(t, owner, data)
	m := map[string]any{"version": version, "license": map[string]any{"content": "CC0-1.0", "commercialUse": true, "attributionRequired": false},
		"taxonomy":  map[string]any{"genres": map[string]any{"primary": "lo-fi"}, "instruments": []string{}, "moods": []string{}},
		"artifacts": []map[string]any{{"kind": "sample", "path": "a.wav", "digest": sum(data), "size": len(data)}}}
	if deps != nil {
		var ds []map[string]string
		for id, rng := range deps {
			ds = append(ds, map[string]string{"id": id, "range": rng})
		}
		m["dependencies"] = ds
	}
	w := r.do(t, req{method: "POST", path: "/api/v1/packages/acme/" + slug + "/releases", bearer: owner, body: map[string]any{"manifest": m}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/" + slug + "/releases/" + version + "/publish", bearer: owner})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestLibraryFlowOverHTTP(t *testing.T) {
	r := newRig(t, nil)
	owner := r.verifiedUser(t, "owner@acme.audio")
	r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: owner, body: map[string]string{"slug": "acme", "name": "Acme"}})
	r.publish(t, owner, "drums", "1.0.0", nil)
	r.publish(t, owner, "drums", "2.0.1", nil)
	r.publish(t, owner, "template", "1.0.0", map[string]string{"acme.drums": "^2.0.0"})

	assert.Equal(t, http.StatusUnauthorized, r.do(t, req{method: "GET", path: "/api/v1/libraries"}).Code)

	user, _ := r.register(t, "producer@example.com")
	w := r.do(t, req{method: "GET", path: "/api/v1/libraries", bearer: user})
	require.Equal(t, http.StatusOK, w.Code)
	libs := decodeList(t, w)
	require.Len(t, libs, 1)
	libID := libs[0].(map[string]any)["id"].(string)
	base := "/api/v1/libraries/" + libID

	w = r.do(t, req{method: "POST", path: base + "/items", bearer: user, body: map[string]any{"packageId": "acme.template"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	item := decode(t, w)
	assert.Equal(t, "acme.template", item["packageId"])
	assert.Equal(t, "1.0.0", item["installedVersion"])
	assert.Equal(t, "Template", item["package"].(map[string]any)["name"])

	// Pin drums to 1.0.0 → conflict with the template's ^2.0.0.
	w = r.do(t, req{method: "POST", path: base + "/items", bearer: user, body: map[string]any{"packageId": "acme.drums", "pinnedVersion": "1.0.0"}})
	require.Equal(t, http.StatusCreated, w.Code)
	w = r.do(t, req{method: "POST", path: base + "/resolve", bearer: user})
	require.Equal(t, http.StatusOK, w.Code)
	conflicts := decode(t, w)["conflicts"].([]any)
	require.Len(t, conflicts, 1)
	assert.Equal(t, "acme.drums", conflicts[0].(map[string]any)["packageId"])
	w = r.do(t, req{method: "POST", path: base + "/lockfiles", bearer: user})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "resolve_conflicts", decode(t, w)["code"])

	// Unpin → resolves cleanly.
	w = r.do(t, req{method: "PATCH", path: base + "/items/acme.drums", bearer: user, body: map[string]any{"pinnedVersion": nil}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Nil(t, decode(t, w)["pinnedVersion"])
	w = r.do(t, req{method: "PATCH", path: base + "/items/acme.drums", bearer: user, body: map[string]any{"upgrade": true}})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "2.0.1", decode(t, w)["installedVersion"])

	w = r.do(t, req{method: "POST", path: base + "/lockfiles", bearer: user})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	saved := decode(t, w)
	lockID := saved["id"].(string)
	lock := saved["lockfile"].(map[string]any)
	assert.EqualValues(t, 1, lock["lockfileVersion"])
	pkgs := lock["packages"].(map[string]any)
	require.Len(t, pkgs, 2)
	drums := pkgs["acme.drums"].(map[string]any)
	assert.Equal(t, "2.0.1", drums["version"])
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, drums["manifestDigest"])

	// The stored bytes download unchanged and hash to the recorded digest.
	w = r.do(t, req{method: "GET", path: "/api/v1/lockfiles/" + lockID, bearer: user})
	require.Equal(t, http.StatusOK, w.Code)
	h := sha256.Sum256(w.Body.Bytes())
	assert.Equal(t, saved["contentDigest"], "sha256:"+hex.EncodeToString(h[:]))
	assert.Equal(t, saved["contentDigest"], w.Header().Get("X-Content-Digest"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "openaudio.lock.json")
	raw := w.Body.Bytes()
	assert.Len(t, decodeList(t, r.do(t, req{method: "GET", path: "/api/v1/lockfiles", bearer: user})), 1)
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/lockfiles/" + lockID, bearer: owner}).Code, "private")
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: base + "/items", bearer: owner}).Code, "private")

	// Import into a second library of another user.
	w = r.do(t, req{method: "POST", path: "/api/v1/libraries", bearer: owner, body: map[string]string{"name": "Studio"}})
	require.Equal(t, http.StatusCreated, w.Code)
	studio := "/api/v1/libraries/" + decode(t, w)["id"].(string)
	w = r.do(t, req{method: "POST", path: studio + "/import", bearer: owner, body: map[string]any{"lockfile": json.RawMessage(raw)}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.EqualValues(t, 2, decode(t, w)["pinned"], "both entries were direct")
	items := decodeList(t, r.do(t, req{method: "GET", path: studio + "/items", bearer: owner}))
	assert.Len(t, items, 2)

	tampered := strings.Replace(string(raw), drums["manifestDigest"].(string), "sha256:"+strings.Repeat("f", 64), 1)
	w = r.do(t, req{method: "POST", path: studio + "/import", bearer: owner, body: `{"lockfile":` + tampered + `}`})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	body := decode(t, w)
	assert.Equal(t, "lockfile_invalid", body["code"])
	assert.Contains(t, body["issues"].([]any)[0].(map[string]any)["message"], "integrity check failed")

	w = r.do(t, req{method: "DELETE", path: base + "/items/acme.drums", bearer: user})
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, http.StatusNoContent, r.do(t, req{method: "DELETE", path: "/api/v1/lockfiles/" + lockID, bearer: user}).Code)
}

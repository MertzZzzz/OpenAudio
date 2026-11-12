package http_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginBundlesOverHTTP(t *testing.T) {
	r := newRig(t, nil)
	owner := r.verifiedUser(t, "dev@acme.audio")
	require.Equal(t, http.StatusCreated, r.do(t, req{method: "POST", path: "/api/v1/publishers", bearer: owner, body: map[string]string{"slug": "acme", "name": "Acme"}}).Code)
	w := r.do(t, req{method: "POST", path: "/api/v1/packages", bearer: owner, body: map[string]string{"publisher": "acme", "slug": "tape", "name": "Tape FX", "kind": "effect"}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	files := map[string][]byte{
		"mac/Tape FX.vst3/Contents/Info.plist":    []byte("<plist/>"),
		"mac/Tape FX.vst3/Contents/MacOS/Tape FX": []byte("\xcf\xfa\xed\xfe mach-o"),
		"linux/tape.clap":                         []byte("\x7fELF clap"),
	}
	var arts []map[string]any
	for p, b := range files {
		r.uploadBlob(t, owner, b)
		arts = append(arts, map[string]any{"kind": "binary", "path": p, "digest": sum(b), "size": len(b)})
	}
	mf := map[string]any{
		"version":  "1.0.0",
		"license":  map[string]any{"content": "MIT", "commercialUse": true, "attributionRequired": false},
		"taxonomy": map[string]any{"genres": map[string]any{"primary": "lo-fi"}, "instruments": []string{}, "moods": []string{"warm"}},
		"module": map[string]any{"moduleClass": "audio-effect", "categories": []string{"saturation"},
			"capabilities": map[string]any{"accepts": []string{"audio"}, "produces": []string{"audio"}, "placement": []string{"insert"}}},
		"artifacts": arts,
		"plugins": []map[string]any{
			{"format": "vst3", "bundle": "mac/Tape FX.vst3", "os": "macos", "arch": []string{"arm64", "x86_64"}},
			{"format": "clap", "bundle": "linux/tape.clap", "os": "linux", "arch": []string{"x86_64"}},
		},
	}
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/tape/releases", bearer: owner, body: map[string]any{"changelog": "First", "manifest": mf}})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	w = r.do(t, req{method: "POST", path: "/api/v1/packages/acme/tape/releases/1.0.0/publish", bearer: owner})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	pkg := decode(t, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape"}))
	plugins := pkg["plugins"].([]any)
	require.Len(t, plugins, 2)
	assert.Equal(t, "macos", plugins[0].(map[string]any)["os"])

	// A folder bundle downloads as one zip rooted at the bundle, executables keep their x bit.
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape/releases/1.0.0/bundles/mac/Tape%20FX.vst3"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "application/zip", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "acme-tape-1.0.0-vst3-macos.zip")
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, err := f.Open()
		require.NoError(t, err)
		got, _ := io.ReadAll(rc)
		rc.Close()
		assert.Equal(t, files["mac/"+f.Name], got, f.Name)
		if f.Name == "Tape FX.vst3/Contents/MacOS/Tape FX" {
			assert.Equal(t, "-rwxr-xr-x", f.Mode().String())
		} else {
			assert.Equal(t, "-rw-r--r--", f.Mode().String())
		}
	}
	sort.Strings(names)
	assert.Equal(t, []string{"Tape FX.vst3/Contents/Info.plist", "Tape FX.vst3/Contents/MacOS/Tape FX"}, names)

	// Single-file CLAP.
	w = r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape/releases/1.0.0/bundles/linux/tape.clap"})
	require.Equal(t, http.StatusOK, w.Code)
	zr, err = zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)
	require.Len(t, zr.File, 1)
	assert.Equal(t, "tape.clap", zr.File[0].Name)

	// Only declared bundles; arbitrary prefixes are not zipped.
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape/releases/1.0.0/bundles/mac"}).Code)
	assert.Equal(t, http.StatusNotFound, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape/releases/9.9.9/bundles/linux/tape.clap"}).Code)

	assert.EqualValues(t, 2, decode(t, r.do(t, req{method: "GET", path: "/api/v1/packages/acme/tape"}))["downloads"])
}

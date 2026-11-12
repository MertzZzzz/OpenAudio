package manifest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginManifest is a cross-platform effect: VST3 for macOS (universal) and Windows, CLAP for macOS and Linux.
func pluginManifest(t *testing.T) map[string]any {
	t.Helper()
	m := parse(t, valid)
	delete(m, "samples")
	delete(m, "musical")
	m["kind"] = "effect"
	m["module"] = map[string]any{
		"moduleClass":  "audio-effect",
		"capabilities": map[string]any{"accepts": []any{"audio"}, "produces": []any{"audio"}, "placement": []any{"insert"}},
		"categories":   []any{"saturation"},
	}
	art := func(p string) map[string]any {
		return map[string]any{"kind": "binary", "path": p, "digest": digest, "size": 10}
	}
	m["artifacts"] = []any{
		art("mac/Tape.vst3/Contents/Info.plist"), art("mac/Tape.vst3/Contents/MacOS/Tape"),
		art("win/Tape.vst3/Contents/x86_64-win/Tape.vst3"), art("win/Tape.vst3/Contents/arm64-win/Tape.vst3"),
		art("mac/Tape.clap/Contents/Info.plist"), art("mac/Tape.clap/Contents/MacOS/Tape"),
		art("linux/Tape.clap"),
		map[string]any{"kind": "preview", "path": "demo.wav", "digest": digest, "size": 10, "contentType": "audio/wav"},
	}
	m["plugins"] = []any{
		map[string]any{"format": "vst3", "bundle": "mac/Tape.vst3", "os": "macos", "arch": []any{"arm64", "x86_64"}, "pluginIds": []any{"0123456789ABCDEF0123456789ABCDEF"}},
		map[string]any{"format": "vst3", "bundle": "win/Tape.vst3", "os": "windows", "arch": []any{"x86_64", "arm64"}},
		map[string]any{"format": "clap", "bundle": "mac/Tape.clap", "os": "macos", "arch": []any{"arm64", "x86_64"}, "pluginIds": []any{"com.acme.tape"}},
		map[string]any{"format": "clap", "bundle": "linux/Tape.clap", "os": "linux", "arch": []any{"x86_64"}},
	}
	return m
}

func plugin(m map[string]any, i int) map[string]any { return m["plugins"].([]any)[i].(map[string]any) }

func TestPluginsValid(t *testing.T) {
	issues := newV(t).Validate(pluginManifest(t))
	assert.Empty(t, issues, issues)
	typed, err := Decode(mustJSON(t, pluginManifest(t)))
	require.NoError(t, err)
	require.Len(t, typed.Plugins, 4)
	assert.Equal(t, []string{"arm64", "x86_64"}, typed.Plugins[0].Arch)
}

func TestPluginRules(t *testing.T) {
	v := newV(t)
	cases := map[string]struct {
		mutate func(m map[string]any)
		path   string
		msg    string
	}{
		"not for sample packs":    {func(m map[string]any) { m["kind"] = "sample-pack"; delete(m, "module") }, "plugins", "only allowed"},
		"unknown format (schema)": {func(m map[string]any) { plugin(m, 0)["format"] = "au" }, "plugins.0.format", ""},
		"extension mismatch":      {func(m map[string]any) { plugin(m, 3)["format"] = "vst3" }, "plugins.3.bundle", "must end in .vst3"},
		"bundle not uploaded":     {func(m map[string]any) { plugin(m, 3)["bundle"] = "linux/Other.clap" }, "plugins.3.bundle", "not in artifacts"},
		"mac single file": {func(m map[string]any) {
			m["artifacts"] = append(m["artifacts"].([]any), map[string]any{"kind": "binary", "path": "Solo.clap", "digest": digest, "size": 1})
			plugin(m, 2)["bundle"] = "Solo.clap"
		}, "plugins.2.bundle", "bundle folder"},
		"mac without executable": {func(m map[string]any) {
			arts := m["artifacts"].([]any)
			m["artifacts"] = append(arts[:1:1], arts[2:]...) // drop mac/Tape.vst3/Contents/MacOS/Tape
		}, "plugins.0.bundle", "Contents/MacOS/ is missing"},
		"windows arch folder missing": {func(m map[string]any) { plugin(m, 1)["arch"] = []any{"x86_64", "arm64", "i386"} }, "plugins.1.arch", "Contents/x86-win/ is missing"},
		"same bundle and OS twice": {func(m map[string]any) {
			plugin(m, 3)["bundle"] = "win/Tape.vst3"
			plugin(m, 3)["format"] = "vst3"
			plugin(m, 3)["os"] = "windows"
			plugin(m, 3)["arch"] = []any{"x86_64"}
		}, "plugins.3.os", "already described"},
		"clap folder on linux": {func(m map[string]any) {
			m["artifacts"] = append(m["artifacts"].([]any), map[string]any{"kind": "binary", "path": "lin/Tape.clap/Tape.so", "digest": digest, "size": 1})
			plugin(m, 3)["bundle"] = "lin/Tape.clap"
		}, "plugins.3.bundle", "single .clap file"},
		"duplicate build": {func(m map[string]any) {
			m["plugins"] = append(m["plugins"].([]any), map[string]any{"format": "clap", "bundle": "mac/Tape.clap", "os": "macos", "arch": []any{"arm64"}})
		}, "plugins.4.arch", "already provided"},
		"bundle file not binary": {func(m map[string]any) {
			m["artifacts"].([]any)[6].(map[string]any)["kind"] = "sample"
		}, "plugins.3.bundle", "must have artifact kind"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := pluginManifest(t)
			c.mutate(m)
			errs := errorsOnly(v.Validate(m))
			require.NotEmpty(t, errs)
			found := false
			for _, e := range errs {
				if (c.path == "" || e.Path == c.path) && (c.msg == "" || contains(e.Message, c.msg) || contains(e.Path, c.msg)) {
					found = true
				}
			}
			assert.True(t, found, "want %s %q in %v", c.path, c.msg, errs)
		})
	}
}

func TestPluginWarnings(t *testing.T) {
	m := pluginManifest(t)
	plugin(m, 2)["pluginIds"] = []any{"tape"}
	m["artifacts"] = append(m["artifacts"].([]any), map[string]any{"kind": "binary", "path": "legacy/Tape.vst3", "digest": digest, "size": 1})
	m["plugins"] = append(m["plugins"].([]any), map[string]any{"format": "vst3", "bundle": "legacy/Tape.vst3", "os": "windows", "arch": []any{"i386"}})
	issues := newV(t).Validate(m)
	assert.Empty(t, errorsOnly(issues), issues)
	assert.Contains(t, paths(issues), "plugins.2.pluginIds")
	assert.Contains(t, paths(issues), "plugins.4.bundle", "legacy single-file VST3")
}

func TestMultiPlatformVST3Bundle(t *testing.T) {
	m := pluginManifest(t)
	art := func(p string) map[string]any {
		return map[string]any{"kind": "binary", "path": p, "digest": digest, "size": 1}
	}
	m["artifacts"] = append(m["artifacts"].([]any), art("all/Tape.vst3/Contents/MacOS/Tape"), art("all/Tape.vst3/Contents/Info.plist"),
		art("all/Tape.vst3/Contents/x86_64-win/Tape.vst3"), art("all/Tape.vst3/Contents/x86_64-linux/Tape.so"))
	m["plugins"] = []any{
		map[string]any{"format": "vst3", "bundle": "all/Tape.vst3", "os": "macos", "arch": []any{"arm64"}},
		map[string]any{"format": "vst3", "bundle": "all/Tape.vst3", "os": "windows", "arch": []any{"x86_64"}},
		map[string]any{"format": "vst3", "bundle": "all/Tape.vst3", "os": "linux", "arch": []any{"x86_64"}},
	}
	assert.Empty(t, errorsOnly(newV(t).Validate(m)))
}

func TestBundleFiles(t *testing.T) {
	files, folder := BundleFiles([]string{"A.vst3/Contents/x", "A.vst3x/y", "B.clap", "A.vst3/Contents/a"}, "A.vst3")
	assert.Equal(t, []string{"A.vst3/Contents/a", "A.vst3/Contents/x"}, files)
	assert.True(t, folder)
	files, folder = BundleFiles([]string{"B.clap"}, "B.clap")
	assert.Equal(t, []string{"B.clap"}, files)
	assert.False(t, folder)
}

func mustJSON(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return b
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

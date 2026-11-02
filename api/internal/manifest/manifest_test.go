package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

var digest = "sha256:" + strings.Repeat("ab", 32)

const valid = `{
  "schema": "openaudio.package/v1",
  "id": "acme.deep-house-toolkit",
  "name": "Deep House Toolkit",
  "kind": "sample-pack",
  "version": "1.2.0",
  "description": "Deep house drums, bass loops and textures.",
  "publisher": { "id": "acme", "name": "Acme Audio" },
  "license": { "content": "CC-BY-4.0", "commercialUse": true, "attributionRequired": true },
  "taxonomy": {
    "genres": { "primary": "deep-house", "secondary": ["tech-house"] },
    "instruments": ["drums", "bass", "percussion"],
    "moods": ["warm", "hypnotic"],
    "useCases": ["club", "studio-production"],
    "energy": 0.62
  },
  "musical": { "bpm": { "value": 124, "range": [122, 126], "sync": true }, "key": { "tonic": "F", "mode": "minor", "camelot": "4A" }, "timeSignature": "4/4" },
  "technical": { "sampleRate": 48000, "bitDepth": 24, "formats": ["wav", "flac"] },
  "artifacts": [
    { "kind": "sample", "path": "loops/bass/deep-bass-loop-124-fm.wav", "digest": "DIGEST", "size": 18342144, "contentType": "audio/wav" }
  ],
  "samples": [{ "id": "deep-bass-loop-124-fm", "file": "loops/bass/deep-bass-loop-124-fm.wav", "contentType": "loop", "instrument": "bass", "bpm": 124 }],
  "tags": { "free": ["analog", "tape", "rolling"] },
  "metadataProvenance": { "bpm": { "source": "auto", "confidence": 0.97 }, "key": { "source": "author" } }
}`

func parse(t *testing.T, s string) map[string]any {
	t.Helper()
	m, err := Parse([]byte(strings.ReplaceAll(s, "DIGEST", digest)))
	require.NoError(t, err)
	return m
}

func errorsOnly(is []domain.Issue) []domain.Issue {
	var out []domain.Issue
	for _, i := range is {
		if i.Severity == "error" {
			out = append(out, i)
		}
	}
	return out
}

func paths(is []domain.Issue) []string {
	var out []string
	for _, i := range is {
		out = append(out, i.Path)
	}
	return out
}

func newV(t *testing.T) *Validator {
	v, err := NewValidator()
	require.NoError(t, err)
	return v
}

func TestSpecExampleIsValid(t *testing.T) {
	issues := newV(t).Validate(parse(t, valid))
	assert.Empty(t, errorsOnly(issues), issues)
}

func TestSchemaErrors(t *testing.T) {
	v := newV(t)
	m := parse(t, valid)
	m["version"] = "1.2"
	m["unknown"] = true
	m["taxonomy"].(map[string]any)["moods"] = []any{"a", "b", "c", "d"}
	m["artifacts"].([]any)[0].(map[string]any)["digest"] = "md5:abc"
	issues := v.Validate(m)
	require.NotEmpty(t, issues)
	for _, i := range issues {
		assert.Equal(t, "error", i.Severity)
	}
	ps := strings.Join(paths(issues), " ")
	for _, want := range []string{"version", "taxonomy.moods", "artifacts.0.digest", "(root)"} {
		assert.Contains(t, ps, want)
	}
}

func TestModuleRequiredForModuleKinds(t *testing.T) {
	v := newV(t)
	m := parse(t, valid)
	m["kind"] = "effect"
	assert.Contains(t, issueText(v.Validate(m)), "module")

	m = parse(t, valid)
	m["module"] = map[string]any{"moduleClass": "audio-effect", "categories": []any{"reverb"},
		"capabilities": map[string]any{"accepts": []any{"audio"}, "produces": []any{"audio"}, "placement": []any{"insert"}}}
	assert.NotEmpty(t, errorsOnly(v.Validate(m)), "sample-pack must not define module")
}

func issueText(is []domain.Issue) string {
	var sb strings.Builder
	for _, i := range is {
		sb.WriteString(i.Path + ": " + i.Message + "\n")
	}
	return sb.String()
}

func TestModuleClassRules(t *testing.T) {
	v := newV(t)
	mk := func(class string, accepts, produces []any) map[string]any {
		m := parse(t, valid)
		m["kind"] = "effect"
		delete(m, "samples")
		m["module"] = map[string]any{"moduleClass": class, "categories": []any{"x"},
			"capabilities": map[string]any{"accepts": accepts, "produces": produces, "placement": []any{"insert"}}}
		return m
	}
	assert.Empty(t, errorsOnly(v.Validate(mk("audio-effect", []any{"audio"}, []any{"audio"}))))
	assert.Contains(t, issueText(v.Validate(mk("audio-effect", []any{"midi"}, []any{"audio"}))), "audio-effect must accept audio")
	assert.Contains(t, issueText(v.Validate(mk("instrument", []any{"audio"}, []any{"midi"}))), "instrument must produce audio")
	assert.Empty(t, errorsOnly(v.Validate(mk("note-generator", []any{}, []any{"midi"}))), "note-generator needs no input")
	assert.Empty(t, errorsOnly(v.Validate(mk("analyzer", []any{"audio"}, []any{"analysis"}))), "analyzer needs no audio out")
}

func TestArtifactPathSafety(t *testing.T) {
	v := newV(t)
	for _, bad := range []string{"../etc/passwd", "a/../../b", "a//b", "./a", "manifest.json"} {
		m := parse(t, valid)
		delete(m, "samples")
		m["artifacts"].([]any)[0].(map[string]any)["path"] = bad
		assert.NotEmpty(t, errorsOnly(v.Validate(m)), bad)
	}
	m := parse(t, valid)
	arts := m["artifacts"].([]any)
	m["artifacts"] = append(arts, arts[0])
	assert.Contains(t, issueText(v.Validate(m)), "duplicate path")

	m = parse(t, valid)
	m["samples"].([]any)[0].(map[string]any)["file"] = "missing.wav"
	assert.Contains(t, issueText(v.Validate(m)), "not listed in artifacts")
}

func TestRecommendationsAreNotErrors(t *testing.T) {
	m := parse(t, valid)
	delete(m, "description")
	m["taxonomy"].(map[string]any)["moods"] = []any{}
	m["musical"].(map[string]any)["bpm"].(map[string]any)["value"] = 130
	issues := newV(t).Validate(m)
	assert.Empty(t, errorsOnly(issues))
	assert.ElementsMatch(t, []string{"description", "taxonomy.moods", "musical.bpm.value"}, paths(issues))
}

func TestNormalize(t *testing.T) {
	target := Target{PackageID: "acme.kit", PublisherID: "acme", PublisherName: "Acme Audio", PackageName: "Kit", Kind: "sample-pack", Version: "1.0.0"}
	m := map[string]any{"publisher": map[string]any{"name": "spoofed"}}
	assert.Empty(t, Normalize(m, target))
	assert.Equal(t, "acme.kit", m["id"])
	assert.Equal(t, SchemaID, m["schema"])
	assert.Equal(t, "1.0.0", m["version"])
	assert.Equal(t, "Kit", m["name"])
	assert.Equal(t, map[string]any{"id": "acme", "name": "Acme Audio"}, m["publisher"])

	conflict := map[string]any{"id": "other.kit", "kind": "effect", "version": "2.0.0", "publisher": map[string]any{"id": "evil"}}
	assert.ElementsMatch(t, []string{"id", "kind", "version", "publisher.id"}, paths(Normalize(conflict, target)))
}

func TestCanonicalDigestIsStable(t *testing.T) {
	a, _ := Parse([]byte(`{"b": 1.50, "a": {"y": [3, 2], "x": "é<>"}}`))
	b, _ := Parse([]byte("{\n  \"a\": {\"x\": \"é<>\", \"y\": [3,2]},\n  \"b\": 1.50\n}"))
	ca, da, err := Canonical(a)
	require.NoError(t, err)
	cb, db, _ := Canonical(b)
	assert.Equal(t, `{"a":{"x":"é<>","y":[3,2]},"b":1.50}`, string(ca), "sorted keys, numbers verbatim, no HTML escaping")
	assert.Equal(t, ca, cb)
	assert.Equal(t, da, db)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, da)
}

func TestParseRejectsNonObjects(t *testing.T) {
	for _, s := range []string{`[]`, `"x"`, `{} {}`, `{`} {
		_, err := Parse([]byte(s))
		assert.Error(t, err, s)
	}
}

func TestTerms(t *testing.T) {
	m, err := decodeTyped(parse(t, valid))
	require.NoError(t, err)
	terms := m.Terms()
	assert.Contains(t, terms, TaxonomyTerm{"genre", "deep-house", 1})
	assert.Contains(t, terms, TaxonomyTerm{"genre", "tech-house", 0.5})
	assert.Contains(t, terms, TaxonomyTerm{"instrument", "drums", 1})
	assert.Contains(t, terms, TaxonomyTerm{"instrument", "bass", 0.5})
	assert.Contains(t, terms, TaxonomyTerm{"mood", "warm", 1})
	assert.Len(t, terms, 9)
}

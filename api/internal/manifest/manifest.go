// Package manifest validates, normalizes and fingerprints `openaudio.package/v1` manifests.
//
// Validation has two layers:
//  1. the JSON Schema in openaudio.package.v1.json (structure, enums, formats);
//  2. domain rules that JSON Schema cannot express well (module class I/O rules from
//     spec §5.4, unsafe artifact paths, duplicates, metadata recommendations from §7.3).
package manifest

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

const SchemaID = "openaudio.package/v1"

//go:embed openaudio.package.v1.json
var SchemaJSON []byte

// MaxSize bounds the raw manifest size accepted by the API.
const MaxSize = 4 << 20

type Validator struct{ schema *jsonschema.Schema }

func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse manifest schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	const url = "https://openaudio.dev/schemas/openaudio.package/v1.json"
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile manifest schema: %w", err)
	}
	return &Validator{schema: s}, nil
}

// Parse decodes a manifest object, keeping numbers exact (json.Number).
func Parse(raw []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("manifest is not valid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("manifest must be a single JSON object")
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("manifest must be a JSON object")
	}
	return m, nil
}

func issue(sev, path, format string, args ...any) domain.Issue {
	return domain.Issue{Severity: sev, Path: path, Message: fmt.Sprintf(format, args...)}
}

// Validate runs the JSON Schema and, if the structure is valid, the domain rules.
// Issues are sorted errors first, then by path.
func (v *Validator) Validate(m map[string]any) []domain.Issue {
	var issues []domain.Issue
	if err := v.schema.Validate(m); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return []domain.Issue{issue("error", "(root)", "%v", err)}
		}
		issues = schemaIssues(ve)
	} else {
		typed, err := decodeTyped(m)
		if err != nil {
			issues = append(issues, issue("error", "(root)", "%v", err))
		} else {
			issues = append(issues, Rules(typed)...)
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		ri, rj := rank(issues[i].Severity), rank(issues[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return issues[i].Path < issues[j].Path
	})
	return issues
}

func rank(s string) int {
	switch s {
	case "error":
		return 0
	case "warning":
		return 1
	}
	return 2
}

// schemaIssues collects the leaves of the validation error tree (the actual failed
// keywords) with dotted instance paths.
func schemaIssues(ve *jsonschema.ValidationError) []domain.Issue {
	printer := message.NewPrinter(language.English)
	seen := map[string]bool{}
	var issues []domain.Issue
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		path := pointerToPath("/" + strings.Join(e.InstanceLocation, "/"))
		msg := e.ErrorKind.LocalizedString(printer)
		if key := path + "\x00" + msg; !seen[key] {
			seen[key] = true
			issues = append(issues, issue("error", path, "%s", msg))
		}
	}
	walk(ve)
	return issues
}

// pointerToPath turns "/artifacts/0/digest" into "artifacts.0.digest".
func pointerToPath(p string) string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "(root)"
	}
	p = strings.ReplaceAll(p, "/", ".")
	return strings.NewReplacer("~1", "/", "~0", "~").Replace(p)
}

func decodeTyped(m map[string]any) (*Manifest, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var t Manifest
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

/* ------------------------------------------------------------ normalize */

// Target is the package a manifest is being attached to.
type Target struct {
	PackageID     string // canonical "<publisher-slug>.<package-slug>"
	PublisherID   string // publisher slug
	PublisherName string
	PackageName   string
	Kind          string
	Version       string // from the URL / request; "" to take it from the manifest
}

// Normalize fills identity fields the server owns (schema, id, publisher, name, kind,
// version) and reports conflicts when the client sent different values.
func Normalize(m map[string]any, t Target) []domain.Issue {
	var issues []domain.Issue
	setOrCheck := func(key, want string) {
		got, present := m[key]
		if !present || got == "" {
			m[key] = want
			return
		}
		if s, ok := got.(string); !ok || s != want {
			issues = append(issues, issue("error", key, "must be %q for this package, got %v", want, got))
		}
	}
	setOrCheck("schema", SchemaID)
	setOrCheck("id", t.PackageID)
	setOrCheck("kind", t.Kind)
	if t.Version != "" {
		setOrCheck("version", t.Version)
	}
	if _, ok := m["name"]; !ok {
		m["name"] = t.PackageName
	}
	switch p := m["publisher"].(type) {
	case nil:
		m["publisher"] = map[string]any{"id": t.PublisherID, "name": t.PublisherName}
	case map[string]any:
		if id, ok := p["id"]; ok && id != t.PublisherID {
			issues = append(issues, issue("error", "publisher.id", "must be %q, got %v", t.PublisherID, id))
		}
		p["id"], p["name"] = t.PublisherID, t.PublisherName
	default:
		issues = append(issues, issue("error", "publisher", "must be an object"))
	}
	return issues
}

/* ------------------------------------------------------------ canonical */

// Canonical serialises the manifest deterministically (object keys sorted, no
// insignificant whitespace, numbers kept verbatim) and returns its sha256 digest.
// The same bytes are stored in PostgreSQL and in S3, so the digest can be re-checked anywhere.
func Canonical(m map[string]any) ([]byte, string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, "", err
	}
	b := bytes.TrimRight(buf.Bytes(), "\n")
	sum := sha256.Sum256(b)
	return b, "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Decode parses canonical bytes into the typed view (no validation).
func Decode(raw []byte) (*Manifest, error) {
	var t Manifest
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

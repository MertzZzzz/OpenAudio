package manifest

import (
	"fmt"
	"path"
	"strings"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/semver"
)

// Rules applies domain checks to a schema-valid manifest.
func Rules(m *Manifest) []domain.Issue {
	var out []domain.Issue
	out = append(out, moduleRules(m.Module)...)
	out = append(out, artifactRules(m)...)
	out = append(out, recommendations(m)...)
	for i, d := range m.Deps {
		p := fmt.Sprintf("dependencies.%d", i)
		if d.ID == m.ID {
			out = append(out, issue("error", p+".id", "a package cannot depend on itself"))
		}
		if !semver.ValidRange(d.Range) {
			out = append(out, issue("error", p+".range", "invalid version range %q", d.Range))
		}
	}
	return out
}

var eventInputs = []string{"midi", "transport"}

func has(list []string, want ...string) bool {
	for _, a := range list {
		for _, b := range want {
			if a == b {
				return true
			}
		}
	}
	return false
}

// moduleRules implements spec §5.4. Ports count as well as declared capabilities.
func moduleRules(mod *Module) []domain.Issue {
	if mod == nil {
		return nil
	}
	in, outSig := append([]string(nil), mod.Capabilities.Accepts...), append([]string(nil), mod.Capabilities.Produces...)
	for _, p := range mod.Ports {
		if p.Direction == "in" {
			in = append(in, p.Signal)
		} else {
			outSig = append(outSig, p.Signal)
		}
	}
	var issues []domain.Issue
	need := func(ok bool, msg string) {
		if !ok {
			issues = append(issues, issue("error", "module.capabilities", "%s", msg))
		}
	}
	switch mod.ModuleClass {
	case "instrument":
		need(has(in, eventInputs...), "instrument must accept an event input (midi or transport)")
		need(has(outSig, "audio"), "instrument must produce audio")
	case "audio-effect":
		need(has(in, "audio"), "audio-effect must accept audio")
		need(has(outSig, "audio"), "audio-effect must produce audio")
	case "note-effect":
		need(has(in, "midi"), "note-effect must accept midi")
		need(has(outSig, "midi"), "note-effect must produce midi")
	case "note-generator":
		need(has(outSig, "midi"), "note-generator must produce midi")
	case "audio-generator":
		need(has(outSig, "audio"), "audio-generator must produce audio")
	}
	if len(mod.Categories) == 0 {
		issues = append(issues, issue("warning", "module.categories", "add at least one category"))
	}
	return issues
}

func artifactRules(m *Manifest) []domain.Issue {
	var issues []domain.Issue
	paths := map[string]int{}
	for i, a := range m.Artifacts {
		p := fmt.Sprintf("artifacts.%d.path", i)
		clean := path.Clean(a.Path)
		if strings.HasPrefix(a.Path, "/") || clean != a.Path || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(a.Path, "\x00") {
			issues = append(issues, issue("error", p, "path must be relative and normalised (no '..', '//' or './'), got %q", a.Path))
		}
		if a.Path == "manifest.json" {
			issues = append(issues, issue("error", p, "manifest.json is reserved"))
		}
		if j, dup := paths[a.Path]; dup {
			issues = append(issues, issue("error", p, "duplicate path (also artifacts.%d)", j))
		}
		paths[a.Path] = i
	}
	for i, s := range m.Samples {
		if _, ok := paths[s.File]; !ok {
			issues = append(issues, issue("error", fmt.Sprintf("samples.%d.file", i), "%q is not listed in artifacts", s.File))
		}
	}
	return issues
}

func recommendations(m *Manifest) []domain.Issue {
	var issues []domain.Issue
	if strings.TrimSpace(m.Description) == "" {
		issues = append(issues, issue("warning", "description", "description is empty"))
	}
	if len(m.Taxonomy.Moods) == 0 {
		issues = append(issues, issue("warning", "taxonomy.moods", "add 1–3 moods to improve discoverability"))
	}
	if has(m.Taxonomy.Genres.Secondary, m.Taxonomy.Genres.Primary) {
		issues = append(issues, issue("warning", "taxonomy.genres.secondary", "repeats the primary genre"))
	}
	if m.Musical != nil && m.Musical.BPM != nil && len(m.Musical.BPM.Range) == 2 {
		lo, hi, v := m.Musical.BPM.Range[0], m.Musical.BPM.Range[1], m.Musical.BPM.Value
		if lo > hi {
			issues = append(issues, issue("error", "musical.bpm.range", "range min must be ≤ max"))
		} else if v < lo || v > hi {
			issues = append(issues, issue("warning", "musical.bpm.value", "value %d is outside the range %d–%d", v, lo, hi))
		}
	}
	if m.Kind == "sample-pack" && (m.Musical == nil || m.Musical.BPM == nil) {
		issues = append(issues, issue("info", "musical.bpm", "no package-level BPM; per-sample BPM will be used"))
	}
	return issues
}

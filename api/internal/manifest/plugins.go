package manifest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
)

// PluginKinds are the package kinds that may ship VST3/CLAP builds.
var PluginKinds = []string{"instrument", "effect", "midi-tool", "utility"}

// vst3ArchDirs maps manifest arch names to the VST3 bundle sub-folders (VST3 SDK "Plug-in Format Structure").
var vst3ArchDirs = map[string]map[string]string{
	"windows": {"x86_64": "x86_64-win", "arm64": "arm64-win", "arm64ec": "arm64ec-win", "i386": "x86-win"},
	"linux":   {"x86_64": "x86_64-linux", "arm64": "aarch64-linux", "i386": "i386-linux"},
}

var (
	clapID = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)+$`)
	vst3ID = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)
)

// BundleFiles returns the artifact paths that make up a bundle: the path itself (single-file
// plug-in) or everything under "bundle/" (folder bundle).
func BundleFiles(paths []string, bundle string) (files []string, folder bool) {
	prefix := bundle + "/"
	for _, p := range paths {
		switch {
		case p == bundle:
			files = append(files, p)
		case strings.HasPrefix(p, prefix):
			files = append(files, p)
			folder = true
		}
	}
	sort.Strings(files)
	return files, folder
}

// pluginRules checks VST3/CLAP builds against the artifacts actually shipped.
func pluginRules(m *Manifest) []domain.Issue {
	if len(m.Plugins) == 0 {
		return nil
	}
	var issues []domain.Issue
	if !has(PluginKinds, m.Kind) {
		return []domain.Issue{issue("error", "plugins", "plug-in builds are only allowed for %s packages", strings.Join(PluginKinds, ", "))}
	}
	paths := make([]string, len(m.Artifacts))
	kinds := map[string]string{}
	for i, a := range m.Artifacts {
		paths[i] = a.Path
		kinds[a.Path] = a.Kind
	}
	type slot struct{ format, os, arch string }
	taken := map[slot]int{}
	bundles := map[string]int{}

	for i, p := range m.Plugins {
		at := func(field string) string { return fmt.Sprintf("plugins.%d.%s", i, field) }
		if ext := strings.ToLower(p.Bundle[strings.LastIndex(p.Bundle, ".")+1:]); ext != p.Format {
			issues = append(issues, issue("error", at("bundle"), "a %s bundle must end in .%s, got %q", p.Format, p.Format, p.Bundle))
		}
		// One VST3 bundle may serve several OSes (Contents/MacOS + Contents/x86_64-win …): one entry per OS.
		if j, dup := bundles[p.Bundle+"\x00"+p.OS]; dup {
			issues = append(issues, issue("error", at("os"), "%s for %s is already described by plugins.%d", p.Bundle, p.OS, j))
		}
		bundles[p.Bundle+"\x00"+p.OS] = i
		for key, j := range bundles {
			other, _, _ := strings.Cut(key, "\x00")
			if j != i && (strings.HasPrefix(p.Bundle, other+"/") || strings.HasPrefix(other, p.Bundle+"/")) {
				issues = append(issues, issue("error", at("bundle"), "bundles cannot be nested (plugins.%d is %q)", j, other))
			}
		}

		files, folder := BundleFiles(paths, p.Bundle)
		if len(files) == 0 {
			issues = append(issues, issue("error", at("bundle"), "%q is not in artifacts: upload the whole %s folder (or file)", p.Bundle, p.Bundle))
			continue
		}
		for _, f := range files {
			if kinds[f] != "binary" {
				issues = append(issues, issue("error", at("bundle"), "%q is part of a plug-in bundle and must have artifact kind \"binary\"", f))
				break
			}
		}
		issues = append(issues, layoutRules(p, files, folder, at)...)

		for _, a := range p.Arch {
			k := slot{p.Format, p.OS, a}
			if j, dup := taken[k]; dup {
				issues = append(issues, issue("error", at("arch"), "%s for %s/%s is already provided by plugins.%d", p.Format, p.OS, a, j))
			}
			taken[k] = i
		}
		for _, id := range p.PluginIDs {
			if p.Format == "clap" && !clapID.MatchString(id) {
				issues = append(issues, issue("warning", at("pluginIds"), "CLAP ids are usually reverse-DNS (com.vendor.plugin), got %q", id))
			}
			if p.Format == "vst3" && !vst3ID.MatchString(id) {
				issues = append(issues, issue("warning", at("pluginIds"), "VST3 class ids are 32 hex digits, got %q", id))
			}
		}
	}
	return issues
}

// layoutRules checks the on-disk shape each host expects for a format/OS pair.
func layoutRules(p Plugin, files []string, folder bool, at func(string) string) []domain.Issue {
	var issues []domain.Issue
	under := func(sub string) bool {
		prefix := p.Bundle + "/" + sub
		for _, f := range files {
			if strings.HasPrefix(f, prefix) {
				return true
			}
		}
		return false
	}
	switch {
	case p.OS == "macos":
		if !folder {
			return append(issues, issue("error", at("bundle"), "on macOS a %s plug-in is a bundle folder (%s/Contents/MacOS/…); upload the folder, not a single file", strings.ToUpper(p.Format), p.Bundle))
		}
		if !under("Contents/MacOS/") {
			issues = append(issues, issue("error", at("bundle"), "%s/Contents/MacOS/ is missing: the bundle has no executable", p.Bundle))
		}
		if !under("Contents/Info.plist") {
			issues = append(issues, issue("warning", at("bundle"), "%s/Contents/Info.plist is missing; hosts may not load the bundle", p.Bundle))
		}
	case p.Format == "clap":
		if folder {
			issues = append(issues, issue("error", at("bundle"), "on %s a CLAP plug-in is a single .clap file, not a folder", p.OS))
		}
		if len(p.Arch) > 1 {
			issues = append(issues, issue("warning", at("arch"), "a single %s .clap file targets one architecture; ship one build per architecture", p.OS))
		}
	case p.Format == "vst3":
		if !folder {
			if p.OS == "linux" {
				return append(issues, issue("error", at("bundle"), "on Linux a VST3 plug-in is a bundle folder (%s/Contents/<arch>-linux/…)", p.Bundle))
			}
			return append(issues, issue("warning", at("bundle"), "single-file VST3 is the legacy layout; current hosts expect a bundle folder (%s/Contents/x86_64-win/…)", p.Bundle))
		}
		for _, a := range p.Arch {
			dir, ok := vst3ArchDirs[p.OS][a]
			if !ok {
				issues = append(issues, issue("error", at("arch"), "%s is not a VST3 architecture on %s", a, p.OS))
				continue
			}
			if !under("Contents/" + dir + "/") {
				issues = append(issues, issue("error", at("arch"), "%s declared but %s/Contents/%s/ is missing", a, p.Bundle, dir))
			}
		}
	}
	return issues
}

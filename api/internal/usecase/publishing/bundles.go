package publishing

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/manifest"
)

// PluginBundle is one VST3/CLAP build of a release, ready to be streamed as a zip.
type PluginBundle struct {
	Plugin manifest.Plugin
	// OSes the bundle serves (a VST3 bundle may hold several platforms).
	OSes []string
	// Name is the bundle's own name ("Tape.vst3"), the root folder inside the zip.
	Name  string
	Files []domain.Artifact
	Size  int64
}

// ZipName is the download file name, e.g. "acme-tape-1.2.0-vst3-macos.zip".
func (b *PluginBundle) zipName(v *domain.PackageView, version string) string {
	return fmt.Sprintf("%s-%s-%s-%s-%s.zip", v.PublisherSlug, v.Slug, version, b.Plugin.Format, strings.Join(b.OSes, "-"))
}

// PluginBundle finds the build whose bundle path is bundlePath in a published (or yanked) release
// and counts the download. Only bundles declared in the manifest's plugins[] can be fetched this way.
func (s *Service) PluginBundle(ctx context.Context, actor Actor, publisherSlug, slug, version, bundlePath string) (*PluginBundle, string, error) {
	v, r, err := s.visibleRelease(ctx, actor, publisherSlug, slug, version)
	if err != nil {
		return nil, "", err
	}
	if r.Status == domain.ReleaseDraft {
		return nil, "", domain.NotFound("bundle_not_found", "drafts have no downloadable files yet")
	}
	m, err := manifest.Decode(r.Manifest)
	if err != nil {
		return nil, "", internal(err)
	}
	var plugin *manifest.Plugin
	var oses []string
	for i := range m.Plugins {
		if m.Plugins[i].Bundle == bundlePath {
			if plugin == nil {
				plugin = &m.Plugins[i]
			}
			oses = append(oses, m.Plugins[i].OS)
		}
	}
	if plugin == nil {
		return nil, "", domain.NotFound("bundle_not_found", "this release has no plug-in bundle at that path")
	}
	arts, err := s.Releases.Artifacts(ctx, r.ID)
	if err != nil {
		return nil, "", internal(err)
	}
	b := &PluginBundle{Plugin: *plugin, OSes: oses, Name: path.Base(plugin.Bundle)}
	for _, a := range arts {
		if a.Path == plugin.Bundle || strings.HasPrefix(a.Path, plugin.Bundle+"/") {
			b.Files = append(b.Files, a)
			b.Size += a.SizeBytes
		}
	}
	if len(b.Files) == 0 {
		return nil, "", domain.NotFound("bundle_not_found", "the bundle's files are missing from this release")
	}
	if err := s.Packages.CountDownload(ctx, v.ID); err != nil {
		s.Logger.WarnContext(ctx, "count download", "package", v.CanonicalID(), "err", err)
	}
	return b, b.zipName(v, r.Version), nil
}

// WriteZip streams the bundle as a zip whose root is the bundle itself (Tape.vst3/Contents/…),
// so unpacking it into a plug-in folder installs it. Executables keep their x bit.
func (s *Service) WriteZip(ctx context.Context, w io.Writer, b *PluginBundle) error {
	zw := zip.NewWriter(w)
	for _, a := range b.Files {
		name := b.Name + strings.TrimPrefix(a.Path, b.Plugin.Bundle) // "Tape.vst3" + "/Contents/…" or just "Tape.clap"
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: a.CreatedAt}
		h.SetMode(0o644)
		if executable(name) {
			h.SetMode(0o755)
		}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		rc, err := s.Blobs.Open(ctx, a.StorageKey)
		if err != nil {
			return fmt.Errorf("open %s: %w", a.Path, err)
		}
		_, err = io.Copy(fw, rc)
		rc.Close()
		if err != nil {
			return fmt.Errorf("copy %s: %w", a.Path, err)
		}
	}
	return zw.Close()
}

// executable reports files a host loads as code: Mach-O executables in Contents/MacOS, Linux shared
// objects and single-file plug-ins.
func executable(name string) bool {
	return strings.Contains(name, "/Contents/MacOS/") || strings.Contains(name, "-linux/") ||
		(!strings.Contains(name, "/") && (strings.HasSuffix(name, ".clap") || strings.HasSuffix(name, ".vst3")))
}

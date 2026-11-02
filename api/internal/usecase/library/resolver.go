package library

import (
	"context"
	"fmt"
	"sort"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/semver"
)

// PackageInfo is what the resolver needs to know about one package.
type PackageInfo struct {
	ID       string // canonical "<publisher>.<package>"
	Releases []ReleaseInfo
}

type ReleaseInfo struct {
	Version        string
	Status         domain.ReleaseStatus
	ManifestDigest string
	Dependencies   []Dependency // declared by this version's manifest
}

type Dependency struct {
	ID    string
	Range string
}

// Loader fetches package information by canonical id; (nil, nil) means "no such package".
type Loader func(ctx context.Context, id string) (*PackageInfo, error)

// Root is a library item.
type Root struct {
	ID        string
	Pinned    string // exact version or ""
	Installed string // preferred version or ""
}

type Request struct {
	From  string `json:"packageId"` // requesting package id, or "library"
	Range string `json:"range"`
}

type Conflict struct {
	PackageID   string    `json:"packageId"`
	Reason      string    `json:"reason"` // no_matching_version | package_not_found
	RequestedBy []Request `json:"requestedBy"`
	Candidates  []string  `json:"candidates"`
}

type Entry struct {
	ID             string
	Version        string
	ManifestDigest string
	Direct         bool
	Yanked         bool
	Dependencies   []Dependency
}

type Resolution struct {
	Entries   map[string]Entry
	Conflicts []Conflict
	Warnings  []domain.Issue
}

const fromLibrary = "library"

// Resolve picks one version per package so that every range requested by the library or by
// a dependency is satisfied:
//
//   - a pinned item is fixed to its exact version (even a yanked one — existing lockfiles keep working);
//   - otherwise the installed version is kept while it still satisfies every range;
//   - otherwise the highest published (never yanked) satisfying version is chosen.
//
// Dependencies come from the manifest of the chosen version, so choosing a different version can
// change the graph; the resolver iterates until the selection is stable.
func Resolve(ctx context.Context, roots []Root, load Loader) (*Resolution, error) {
	cache := map[string]*PackageInfo{}
	get := func(id string) (*PackageInfo, error) {
		if p, ok := cache[id]; ok {
			return p, nil
		}
		p, err := load(ctx, id)
		if err != nil {
			return nil, err
		}
		cache[id] = p
		return p, nil
	}
	preferred := map[string]string{}
	direct := map[string]bool{}
	for _, r := range roots {
		direct[r.ID] = true
		if r.Pinned != "" {
			preferred[r.ID] = r.Pinned
		} else if r.Installed != "" {
			preferred[r.ID] = r.Installed
		}
	}

	chosen := map[string]string{}
	var conflicts []Conflict
	const maxRounds = 25
	stable := false
	for round := 0; round < maxRounds; round++ {
		requests := map[string][]Request{}
		for _, r := range roots {
			rng := "*"
			if r.Pinned != "" {
				rng = r.Pinned
			}
			requests[r.ID] = append(requests[r.ID], Request{From: fromLibrary, Range: rng})
		}
		for id, version := range chosen {
			p, err := get(id)
			if err != nil {
				return nil, err
			}
			for _, d := range release(p, version).Dependencies {
				requests[d.ID] = append(requests[d.ID], Request{From: id, Range: d.Range})
			}
		}

		next := map[string]string{}
		conflicts = nil
		ids := make([]string, 0, len(requests))
		for id := range requests {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			reqs := requests[id]
			p, err := get(id)
			if err != nil {
				return nil, err
			}
			if p == nil {
				conflicts = append(conflicts, Conflict{PackageID: id, Reason: "package_not_found", RequestedBy: reqs, Candidates: []string{}})
				continue
			}
			pinnedYanked := ""
			for _, r := range reqs {
				if r.From == fromLibrary && semver.Valid(r.Range) {
					pinnedYanked = r.Range
				}
			}
			var pool, candidates []string
			for _, rel := range p.Releases {
				usable := rel.Status == domain.ReleasePublished || (rel.Status == domain.ReleaseYanked && rel.Version == pinnedYanked)
				if !usable {
					continue
				}
				candidates = append(candidates, rel.Version)
				ok := true
				for _, r := range reqs {
					if !semver.Satisfies(rel.Version, r.Range) {
						ok = false
						break
					}
				}
				if ok {
					pool = append(pool, rel.Version)
				}
			}
			if len(pool) == 0 {
				if candidates == nil {
					candidates = []string{}
				}
				conflicts = append(conflicts, Conflict{PackageID: id, Reason: "no_matching_version", RequestedBy: reqs, Candidates: candidates})
				continue
			}
			pick := semver.Max(pool)
			if pref := preferred[id]; pref != "" && contains(pool, pref) {
				pick = pref
			}
			next[id] = pick
		}
		if equal(next, chosen) {
			stable = true
			break
		}
		chosen = next
	}

	res := &Resolution{Entries: map[string]Entry{}, Conflicts: conflicts}
	if !stable {
		res.Warnings = append(res.Warnings, domain.Issue{Severity: "warning", Path: "(root)", Message: "dependency graph did not settle; showing the last selection"})
	}
	for id, version := range chosen {
		p, _ := get(id)
		rel := release(p, version)
		e := Entry{ID: id, Version: version, ManifestDigest: rel.ManifestDigest, Direct: direct[id], Yanked: rel.Status == domain.ReleaseYanked, Dependencies: rel.Dependencies}
		if e.Yanked {
			res.Warnings = append(res.Warnings, domain.Issue{Severity: "warning", Path: id, Message: fmt.Sprintf("%s@%s is yanked; it stays installable because it is pinned", id, version)})
		}
		res.Entries[id] = e
	}
	sort.Slice(res.Warnings, func(i, j int) bool { return res.Warnings[i].Path < res.Warnings[j].Path })
	return res, nil
}

func release(p *PackageInfo, version string) ReleaseInfo {
	if p != nil {
		for _, r := range p.Releases {
			if r.Version == version {
				return r
			}
		}
	}
	return ReleaseInfo{Version: version}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// Package semver implements the small subset of semantic versioning OpenAudio needs:
// MAJOR.MINOR.PATCH versions and ranges "*", "1.2.3", "^1.2.3", "~1.2.3", ">=1.2.3".
package semver

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

type Version struct{ Major, Minor, Patch int }

var versionRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

func Parse(s string) (Version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid version %q", s)
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return Version{a, b, c}, nil
}

func Valid(s string) bool { return versionRe.MatchString(s) }

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) }

func (v Version) Compare(o Version) int {
	for _, d := range []int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		if d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Compare orders version strings; invalid versions sort first.
func Compare(a, b string) int {
	va, ea := Parse(a)
	vb, eb := Parse(b)
	switch {
	case ea != nil && eb != nil:
		return 0
	case ea != nil:
		return -1
	case eb != nil:
		return 1
	}
	return va.Compare(vb)
}

// Max returns the highest valid version, or "" when none.
func Max(versions []string) string {
	vs := append([]string(nil), versions...)
	sort.SliceStable(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) < 0 })
	for i := len(vs) - 1; i >= 0; i-- {
		if Valid(vs[i]) {
			return vs[i]
		}
	}
	return ""
}

var rangeRe = regexp.MustCompile(`^(\*|(\^|~|>=)?(\d+\.\d+\.\d+))$`)

func ValidRange(r string) bool {
	m := rangeRe.FindStringSubmatch(r)
	return m != nil && (m[1] == "*" || Valid(m[3]))
}

// Satisfies reports whether version v is inside range r.
func Satisfies(v, r string) bool {
	ver, err := Parse(v)
	if err != nil {
		return false
	}
	m := rangeRe.FindStringSubmatch(r)
	if m == nil {
		return false
	}
	if m[1] == "*" {
		return true
	}
	base, err := Parse(m[3])
	if err != nil {
		return false
	}
	switch m[2] {
	case "":
		return ver.Compare(base) == 0
	case ">=":
		return ver.Compare(base) >= 0
	case "~":
		return ver.Compare(base) >= 0 && ver.Major == base.Major && ver.Minor == base.Minor
	default: // ^
		if ver.Compare(base) < 0 {
			return false
		}
		if base.Major == 0 {
			return ver.Major == 0 && ver.Minor == base.Minor
		}
		return ver.Major == base.Major
	}
}

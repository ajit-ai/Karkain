package pm

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Version struct {
	Major        int
	Minor        int
	Patch        int
	PreRelease   string
	BuildMetadata string
}

var versionRegexp = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9]+(?:\.[a-zA-Z0-9]+)*))?(?:\+([a-zA-Z0-9]+(?:\.[a-zA-Z0-9]+)*))?$`)

func ParseVersion(s string) (Version, error) {
	m := versionRegexp.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("invalid semver: %q", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return Version{
		Major:        major,
		Minor:        minor,
		Patch:        patch,
		PreRelease:   m[4],
		BuildMetadata: m[5],
	}, nil
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	if v.BuildMetadata != "" {
		s += "+" + v.BuildMetadata
	}
	return s
}

func parsePreReleaseIdentifiers(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

func comparePreRelease(a, b string) int {
	ia := parsePreReleaseIdentifiers(a)
	ib := parsePreReleaseIdentifiers(b)

	if len(ia) == 0 && len(ib) == 0 {
		return 0
	}
	if len(ia) == 0 {
		return 1
	}
	if len(ib) == 0 {
		return -1
	}

	for i := 0; i < len(ia) && i < len(ib); i++ {
		na, ea := strconv.Atoi(ia[i])
		nb, eb := strconv.Atoi(ib[i])
		if ea == nil && eb == nil {
			if na < nb {
				return -1
			}
			if na > nb {
				return 1
			}
			continue
		}
		if ea == nil {
			return -1
		}
		if eb == nil {
			return 1
		}
		if ia[i] < ib[i] {
			return -1
		}
		if ia[i] > ib[i] {
			return 1
		}
	}
	if len(ia) < len(ib) {
		return -1
	}
	if len(ia) > len(ib) {
		return 1
	}
	return 0
}

func Compare(a, b Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}
	return comparePreRelease(a.PreRelease, b.PreRelease)
}

type constraint struct {
	min        *Version
	max        *Version
	minIncl    bool
	maxIncl    bool
	exactMatch *Version
}

func parseSingleConstraint(s string) (constraint, error) {
	s = strings.TrimSpace(s)
	if s == "*" || s == "" {
		return constraint{}, nil
	}

	if strings.HasPrefix(s, "^") {
		return parseCaret(strings.TrimPrefix(s, "^"))
	}
	if strings.HasPrefix(s, "~") {
		return parseTilde(strings.TrimPrefix(s, "~"))
	}
	if strings.HasPrefix(s, ">=") {
		v, err := ParseVersion(strings.TrimPrefix(s, ">="))
		if err != nil {
			return constraint{}, err
		}
		return constraint{min: &v, minIncl: true}, nil
	}
	if strings.HasPrefix(s, ">") {
		v, err := ParseVersion(strings.TrimPrefix(s, ">"))
		if err != nil {
			return constraint{}, err
		}
		return constraint{min: &v, minIncl: false}, nil
	}
	if strings.HasPrefix(s, "<=") {
		v, err := ParseVersion(strings.TrimPrefix(s, "<="))
		if err != nil {
			return constraint{}, err
		}
		return constraint{max: &v, maxIncl: true}, nil
	}
	if strings.HasPrefix(s, "<") {
		v, err := ParseVersion(strings.TrimPrefix(s, "<"))
		if err != nil {
			return constraint{}, err
		}
		return constraint{max: &v, maxIncl: false}, nil
	}
	if strings.HasPrefix(s, "=") {
		v, err := ParseVersion(strings.TrimPrefix(s, "="))
		if err != nil {
			return constraint{}, err
		}
		return constraint{exactMatch: &v}, nil
	}

	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) == 3 && (parts[2] == "x" || parts[2] == "X" || parts[2] == "*") {
			minor, err := strconv.Atoi(parts[1])
			if err != nil {
				return constraint{}, fmt.Errorf("invalid wildcard minor: %q", parts[1])
			}
			major, err := strconv.Atoi(parts[0])
			if err != nil {
				return constraint{}, fmt.Errorf("invalid wildcard major: %q", parts[0])
			}
			min := Version{Major: major, Minor: minor, Patch: 0}
			max := Version{Major: major, Minor: minor + 1, Patch: 0}
			return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
		}
		if len(parts) == 3 && (parts[1] == "x" || parts[1] == "X" || parts[1] == "*") {
			major, err := strconv.Atoi(parts[0])
			if err != nil {
				return constraint{}, fmt.Errorf("invalid wildcard major: %q", parts[0])
			}
			min := Version{Major: major, Minor: 0, Patch: 0}
			max := Version{Major: major + 1, Minor: 0, Patch: 0}
			return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
		}
		if len(parts) == 2 && (parts[1] == "x" || parts[1] == "X" || parts[1] == "*") {
			major, err := strconv.Atoi(parts[0])
			if err != nil {
				return constraint{}, fmt.Errorf("invalid wildcard major: %q", parts[0])
			}
			min := Version{Major: major, Minor: 0, Patch: 0}
			max := Version{Major: major + 1, Minor: 0, Patch: 0}
			return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
		}
	}

	v, err := ParseVersion(s)
	if err != nil {
		return constraint{}, err
	}
	return constraint{exactMatch: &v}, nil
}

func parseCaret(s string) (constraint, error) {
	v, err := ParseVersion(s)
	if err != nil {
		return constraint{}, err
	}
	if v.Major != 0 {
		min := Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch}
		max := Version{Major: v.Major + 1, Minor: 0, Patch: 0}
		return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
	}
	if v.Minor != 0 {
		min := Version{Major: 0, Minor: v.Minor, Patch: v.Patch}
		max := Version{Major: 0, Minor: v.Minor + 1, Patch: 0}
		return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
	}
	min := Version{Major: 0, Minor: 0, Patch: v.Patch}
	return constraint{exactMatch: &min}, nil
}

func parseTilde(s string) (constraint, error) {
	v, err := ParseVersion(s)
	if err != nil {
		return constraint{}, err
	}
	min := v
	max := Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0}
	return constraint{min: &min, minIncl: true, max: &max, maxIncl: false}, nil
}

func (c constraint) satisfies(v Version) bool {
	if c.exactMatch != nil {
		return Compare(v, *c.exactMatch) == 0
	}
	if c.min != nil {
		cmp := Compare(v, *c.min)
		if c.minIncl {
			if cmp < 0 {
				return false
			}
		} else {
			if cmp <= 0 {
				return false
			}
		}
	}
	if c.max != nil {
		cmp := Compare(v, *c.max)
		if c.maxIncl {
			if cmp > 0 {
				return false
			}
		} else {
			if cmp >= 0 {
				return false
			}
		}
	}
	return true
}

func Satisfies(v Version, constraintStr string) (bool, error) {
	constraintStr = strings.TrimSpace(constraintStr)
	if constraintStr == "" || constraintStr == "*" {
		return true, nil
	}

	parts := splitConstraint(constraintStr)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		c, err := parseSingleConstraint(part)
		if err != nil {
			return false, fmt.Errorf("invalid constraint %q: %w", part, err)
		}
		if !c.satisfies(v) {
			return false, nil
		}
	}
	return true, nil
}

func splitConstraint(s string) []string {
	s = strings.ReplaceAll(s, "||", "||")
	if strings.Contains(s, "||") {
		return strings.Split(s, "||")
	}
	if strings.Contains(s, ", ") {
		return strings.Split(s, ", ")
	}
	if strings.Contains(s, " ") {
		parts := strings.Fields(s)
		if len(parts) >= 2 && (parts[0] == ">=" || parts[0] == ">" || parts[0] == "<=" || parts[0] == "<") {
			return []string{parts[0] + parts[1]}
		}
	}
	return []string{s}
}

func ParseVersions(ss []string) ([]Version, error) {
	var vs []Version
	for _, s := range ss {
		v, err := ParseVersion(s)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	return vs, nil
}

func LatestSatisfying(versions []Version, constraintStr string) (Version, error) {
	sorted := make([]Version, len(versions))
	copy(sorted, versions)
	sort.Slice(sorted, func(i, j int) bool {
		return Compare(sorted[i], sorted[j]) > 0
	})
	for _, v := range sorted {
		ok, err := Satisfies(v, constraintStr)
		if err != nil {
			return Version{}, err
		}
		if ok {
			return v, nil
		}
	}
	return Version{}, fmt.Errorf("no version satisfies constraint %q", constraintStr)
}

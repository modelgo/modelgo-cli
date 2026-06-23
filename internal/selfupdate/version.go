// Package selfupdate handles npm-registry version checks, install-method
// detection, npm-based self-replacement, binary verification, and skills
// sync for the `modelgo update` command.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// NpmPackage is the published npm package name.
	NpmPackage = "@model-go/cli"
	// SkillsRepo is the source passed to `npx skills add`.
	SkillsRepo = "modelgo/modelgo-cli"
	// RepoURL is the GitHub repository (used for release/changelog links).
	RepoURL = "https://github.com/modelgo/modelgo-cli"

	fetchTimeout = 5 * time.Second
	maxBody      = 256 << 10 // 256 KB
)

// registryURL is the npm "latest" dist-tag endpoint. A var (not const) so
// tests can point it at an httptest server.
var registryURL = "https://registry.npmjs.org/@model-go/cli/latest"

// DefaultClient is the HTTP client used for npm registry requests.
// Override in tests with an httptest server client.
var DefaultClient *http.Client

func httpClient() *http.Client {
	if DefaultClient != nil {
		return DefaultClient
	}
	return &http.Client{Timeout: fetchTimeout}
}

// FetchLatest queries the npm registry and returns the latest published
// version of @model-go/cli (without a leading "v").
func FetchLatest() (string, error) {
	resp, err := httpClient().Get(registryURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", err
	}

	var result struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Version == "" {
		return "", fmt.Errorf("npm registry: empty version")
	}
	return result.Version, nil
}

// ReleaseURL returns the GitHub release page for a version.
func ReleaseURL(version string) string {
	return RepoURL + "/releases/tag/v" + strings.TrimPrefix(version, "v")
}

// ChangelogURL returns the CHANGELOG link.
func ChangelogURL() string { return RepoURL + "/blob/main/CHANGELOG.md" }

// IsNewer reports whether version a should be considered an update over b.
//
// When both parse as semver, standard comparison applies (including
// pre-release ordering: 1.0.0-rc.1 < 1.0.0). When b cannot be parsed (e.g. a
// bare "dev" build), any valid a is considered newer. When a cannot be parsed,
// returns false.
func IsNewer(a, b string) bool {
	ap := parseVersionDetail(a)
	bp := parseVersionDetail(b)
	if ap == nil {
		return false // can't confirm remote is newer
	}
	if bp == nil {
		return true // local version unparseable → assume outdated
	}
	for i := 0; i < 3; i++ {
		if ap.core[i] > bp.core[i] {
			return true
		}
		if ap.core[i] < bp.core[i] {
			return false
		}
	}
	return comparePrerelease(ap.prerelease, bp.prerelease) > 0
}

// ParseVersion parses "X.Y.Z" (optional "v" prefix and pre-release suffix)
// into [major, minor, patch]. Returns nil on invalid input.
func ParseVersion(v string) []int {
	parsed := parseVersionDetail(v)
	if parsed == nil {
		return nil
	}
	return []int{parsed.core[0], parsed.core[1], parsed.core[2]}
}

type parsedVersion struct {
	core       [3]int
	prerelease string
}

// validPrerelease matches semver pre-release identifiers (dot-separated).
var validPrerelease = regexp.MustCompile(
	`^(?:0|[1-9]\d*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*)` +
		`(?:\.(?:0|[1-9]\d*|[0-9]*[a-zA-Z-][0-9a-zA-Z-]*))*$`)

func parseVersionDetail(v string) *parsedVersion {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	prerelease := ""
	if idx := strings.Index(v, "-"); idx >= 0 {
		prerelease = v[idx+1:]
		v = v[:idx]
		if prerelease == "" || !validPrerelease.MatchString(prerelease) {
			return nil
		}
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	var nums [3]int
	for i, p := range parts {
		if len(p) > 1 && p[0] == '0' {
			return nil // leading zero (e.g. "01.0.0")
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return &parsedVersion{core: nums, prerelease: prerelease}
}

func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1 // release > prerelease
	}
	if b == "" {
		return -1
	}
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		if cmp := comparePrereleaseIdentifier(ap[i], bp[i]); cmp != 0 {
			return cmp
		}
	}
	switch {
	case len(ap) > len(bp):
		return 1
	case len(ap) < len(bp):
		return -1
	default:
		return 0
	}
}

func comparePrereleaseIdentifier(a, b string) int {
	an, aErr := strconv.Atoi(a)
	bn, bErr := strconv.Atoi(b)
	aNumeric := aErr == nil
	bNumeric := bErr == nil
	switch {
	case aNumeric && bNumeric:
		switch {
		case an > bn:
			return 1
		case an < bn:
			return -1
		default:
			return 0
		}
	case aNumeric:
		return -1 // numeric < alphanumeric
	case bNumeric:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

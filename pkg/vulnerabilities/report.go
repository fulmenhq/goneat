package vulnerabilities

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Finding struct {
	ID             string   `json:"id"`
	Severity       Severity `json:"severity"`
	SeverityRaw    string   `json:"severity_raw"`
	PackageNames   []string `json:"package_names"`
	PackageCount   int      `json:"package_count"`
	PURLs          []string `json:"purls,omitempty"`
	FixVersions    []string `json:"fix_versions,omitempty"`
	FixState       string   `json:"fix_state,omitempty"`
	PublishedDate  string   `json:"published_date,omitempty"`
	FixFirstSeen   string   `json:"fix_first_seen,omitempty"`
	DataSource     string   `json:"data_source,omitempty"`
	AdvisoryURLs   []string `json:"advisory_urls,omitempty"`
	Suppressed     bool     `json:"suppressed"`
	SuppressReason string   `json:"suppress_reason,omitempty"`
}

type Summary struct {
	SBOMPackages int              `json:"sbom_packages"`
	MatchCount   int              `json:"match_count"`
	Counts       map[Severity]int `json:"counts"`
	Violations   int              `json:"violations"`
	Suppressed   int              `json:"suppressed"`
}

type Report struct {
	Version     string            `json:"version"`
	GeneratedAt time.Time         `json:"generated_at"`
	Target      string            `json:"target"`
	Tool        string            `json:"tool"`
	ToolVersion string            `json:"tool_version"`
	SBOMPath    string            `json:"sbom_path"`
	RawPath     string            `json:"raw_path"`
	Summary     Summary           `json:"summary"`
	Findings    []Finding         `json:"findings"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func PrettyJSON(data []byte) ([]byte, error) {
	var decoded interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return json.MarshalIndent(decoded, "", "  ")
}

type grypeAvailable struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Kind    string `json:"kind"`
}

type grypeReport struct {
	Matches []struct {
		Artifact struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			PURL    string `json:"purl"`
		} `json:"artifact"`
		Vulnerability struct {
			ID            string `json:"id"`
			Severity      string `json:"severity"`
			PublishedDate string `json:"publishedDate"`
			Fix           struct {
				Versions  []string         `json:"versions"`
				State     string           `json:"state"`
				Available []grypeAvailable `json:"available"`
			} `json:"fix"`
			DataSource string   `json:"dataSource"`
			URLs       []string `json:"urls"`
		} `json:"vulnerability"`
	} `json:"matches"`
}

func ParseGrype(raw []byte) ([]Finding, map[Severity]int, error) {
	var report grypeReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, nil, err
	}

	counts := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     0,
		SeverityMedium:   0,
		SeverityLow:      0,
		SeverityUnknown:  0,
	}

	byID := map[string]*Finding{}
	for _, m := range report.Matches {
		id := strings.TrimSpace(m.Vulnerability.ID)
		if id == "" {
			continue
		}
		rawSeverity := m.Vulnerability.Severity
		sev := NormalizeSeverity(rawSeverity)
		counts[sev]++

		f, ok := byID[id]
		if !ok {
			f = &Finding{ID: id, Severity: sev, SeverityRaw: rawSeverity}
			byID[id] = f
		}
		if SeverityMeetsOrExceeds(sev, f.Severity) {
			f.Severity = sev
			f.SeverityRaw = rawSeverity
			f.DataSource = m.Vulnerability.DataSource
			f.AdvisoryURLs = m.Vulnerability.URLs
			f.FixVersions = dedupeStrings(m.Vulnerability.Fix.Versions)
			f.FixState = m.Vulnerability.Fix.State
			f.PublishedDate = strings.TrimSpace(m.Vulnerability.PublishedDate)
			f.FixFirstSeen = firstAvailableDate(m.Vulnerability.Fix.Available)
		}

		pkg := strings.TrimSpace(m.Artifact.Name)
		if pkg != "" {
			f.PackageNames = append(f.PackageNames, pkg)
		}
		purl := strings.TrimSpace(m.Artifact.PURL)
		if purl != "" {
			f.PURLs = append(f.PURLs, purl)
		}
	}

	findings := make([]Finding, 0, len(byID))
	for _, f := range byID {
		f.PackageNames = dedupeStrings(f.PackageNames)
		f.PURLs = dedupeStrings(f.PURLs)
		f.PackageCount = len(f.PackageNames)
		findings = append(findings, *f)
	}

	sort.Slice(findings, func(i, j int) bool {
		order := map[Severity]int{SeverityCritical: 4, SeverityHigh: 3, SeverityMedium: 2, SeverityLow: 1, SeverityUnknown: 0}
		si := order[findings[i].Severity]
		sj := order[findings[j].Severity]
		if si != sj {
			return si > sj
		}
		return findings[i].ID < findings[j].ID
	})

	return findings, counts, nil
}

func firstAvailableDate(items []grypeAvailable) string {
	for _, it := range items {
		if strings.TrimSpace(it.Date) != "" {
			return it.Date
		}
	}
	return ""
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

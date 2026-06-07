// Package sarif renders scan results as SARIF 2.1.0 so they can be uploaded to
// GitHub code scanning or any SARIF-aware security dashboard. Each reachable
// Disallow path becomes a result; obviously-sensitive paths are raised to
// "error" level.
package sarif

import (
	"strings"

	"github.com/zvdy/parsero-go/internal/store"
)

const (
	version = "2.1.0"
	schema  = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
	ruleID  = "exposed-disallow-path"
)

type Report struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []run  `json:"runs"`
}

type run struct {
	Tool    tool     `json:"tool"`
	Results []result `json:"results"`
}

type tool struct {
	Driver driver `json:"driver"`
}

type driver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri"`
	Rules          []rule `json:"rules"`
}

type rule struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ShortDescription textBlock `json:"shortDescription"`
}

type result struct {
	RuleID    string     `json:"ruleId"`
	Level     string     `json:"level"`
	Message   textBlock  `json:"message"`
	Locations []location `json:"locations"`
}

type textBlock struct {
	Text string `json:"text"`
}

type location struct {
	PhysicalLocation physicalLocation `json:"physicalLocation"`
}

type physicalLocation struct {
	ArtifactLocation artifactLocation `json:"artifactLocation"`
}

type artifactLocation struct {
	URI string `json:"uri"`
}

var sensitiveMarkers = []string{
	"admin", "login", "wp-admin", "phpmyadmin", ".git", ".env", "backup",
	"config", "secret", "private", "password", "db", "sql", "dump", ".ssh",
	"credential", "token", "api-key", "apikey", "internal",
}

// Build reports only the reachable (HTTP 200) Disallow paths — what's actually
// accessible is what matters.
func Build(scan store.Scan, rows []store.ResultRow) Report {
	var results []result
	for _, r := range rows {
		if r.StatusCode != 200 {
			continue
		}
		results = append(results, result{
			RuleID:  ruleID,
			Level:   level(r.URL),
			Message: textBlock{Text: "Disallow path is reachable: " + r.URL},
			Locations: []location{{
				PhysicalLocation: physicalLocation{
					ArtifactLocation: artifactLocation{URI: r.URL},
				},
			}},
		})
	}

	return Report{
		Schema:  schema,
		Version: version,
		Runs: []run{{
			Tool: tool{Driver: driver{
				Name:           "parsero",
				InformationURI: "https://github.com/zvdy/parsero-go",
				Rules: []rule{{
					ID:               ruleID,
					Name:             "ExposedDisallowPath",
					ShortDescription: textBlock{Text: "A robots.txt Disallow path is publicly reachable."},
				}},
			}},
			Results: results,
		}},
	}
}

func level(url string) string {
	lower := strings.ToLower(url)
	for _, m := range sensitiveMarkers {
		if strings.Contains(lower, m) {
			return "error"
		}
	}
	return "warning"
}

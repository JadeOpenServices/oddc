package oddc

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Evidence is public. It records what a machine of a model did, never
// which machine or whose. These checks refuse what would identify one.

var (
	// observedDate is a day; a time of day would help tell machines apart.
	observedDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	// resultValue is a short status such as "pass" or "not-exposed".
	resultValue = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,31}$`)

	// identifyingKeys are environment keys, lower case without separators,
	// that name something about one machine or person. identifyingParts
	// do so anywhere in a key.
	identifyingKeys = []string{
		"address", "email", "host", "ip", "location", "login", "mac",
		"mail", "name", "owner", "phone", "user",
	}
	identifyingParts = []string{
		"email", "guid", "hostname", "ipaddr", "macaddr", "machineid",
		"serial", "username", "uuid",
	}

	identifyingValues = []struct {
		what    string
		pattern *regexp.Regexp
	}{
		{"MAC address", regexp.MustCompile(`(?i)\b[0-9a-f]{2}([:-])(?:[0-9a-f]{2}[:-]){4}[0-9a-f]{2}\b`)},
		{"UUID", regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)},
		{"machine ID", regexp.MustCompile(`(?i)\b[0-9a-f]{32}\b`)},
		{"e-mail address", regexp.MustCompile(`[^\s@]+@[^\s@]+\.[^\s@]+`)},
		{"IPv4 address", regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)\b`)},
		{"IPv6 address", regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){2,7}[0-9a-f]{0,4}\b`)},
		{"home directory", regexp.MustCompile(`(?:/home/|/Users/|\\Users\\)[^/\\\s]+`)},
	}
)

// identifyingKey reports whether a key names identifying data, ignoring
// case and separators: "serialNumber", "host_name" and "MAC" all do.
func identifyingKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		switch r {
		case '-', '_', '.', ' ':
			return -1
		}
		return r
	}, strings.ToLower(key))

	for _, word := range identifyingKeys {
		if normalized == word {
			return true
		}
	}

	for _, part := range identifyingParts {
		if strings.Contains(normalized, part) {
			return true
		}
	}

	return false
}

// identifyingValue names the kind of identifying data a value holds, or "".
func identifyingValue(value string) string {
	for _, check := range identifyingValues {
		if check.pattern.MatchString(value) {
			return check.what
		}
	}

	return ""
}

// evidenceProblems lists what in a record could identify a machine or a
// person, or does not have the shape evidence must have.
func evidenceProblems(evidence Evidence) []string {
	var problems []string

	if !observedDate.MatchString(evidence.ObservedAt) {
		problems = append(problems, fmt.Sprintf(
			"observedAt %q must be a date (YYYY-MM-DD)",
			evidence.ObservedAt,
		))
	}

	for _, field := range []struct{ name, value string }{
		{"id", evidence.ID},
		{"observedAt", evidence.ObservedAt},
	} {
		if what := identifyingValue(field.value); what != "" {
			problems = append(problems, fmt.Sprintf("%s holds a %s", field.name, what))
		}
	}

	for _, key := range sortedKeys(evidence.Results) {
		value, ok := evidence.Results[key].(string)
		if !ok || !resultValue.MatchString(value) {
			problems = append(problems, fmt.Sprintf(
				"results.%s must be a short status such as \"pass\", got %v",
				key,
				evidence.Results[key],
			))
		}
	}

	var walk func(path string, value any)
	walk = func(path string, value any) {
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range sortedKeys(typed) {
				if identifyingKey(key) {
					problems = append(problems, fmt.Sprintf(
						"%s.%s names identifying data",
						path,
						key,
					))
					continue
				}
				walk(path+"."+key, typed[key])
			}

		case []any:
			for i, item := range typed {
				walk(fmt.Sprintf("%s[%d]", path, i), item)
			}

		case string:
			if what := identifyingValue(typed); what != "" {
				problems = append(problems, fmt.Sprintf("%s holds a %s", path, what))
			}
		}
	}
	walk("environment", evidence.Environment)
	walk("results", evidence.Results)

	return problems
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

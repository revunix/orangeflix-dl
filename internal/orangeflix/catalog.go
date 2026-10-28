package orangeflix

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatByPID returns the catalog entry of a format id.
func FormatByPID(pid int) (Format, bool) {
	for _, format := range Catalog {
		if format.PID == pid {
			return format, true
		}
	}
	return Format{}, false
}

// FormatBySlug returns the catalog entry of a slug.
func FormatBySlug(slug string) (Format, bool) {
	lowered := strings.ToLower(strings.TrimSpace(slug))
	for _, format := range Catalog {
		if strings.ToLower(format.Slug) == lowered {
			return format, true
		}
	}
	return Format{}, false
}

// LookupFormat resolves a format by its slug, its title or its numeric id.
// Formats that are not in the catalog are accepted as well: the API only
// needs the id.
func LookupFormat(value string) (Format, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Format{}, fmt.Errorf("no format given")
	}
	if format, ok := FormatBySlug(trimmed); ok {
		return format, nil
	}
	if pid, err := strconv.Atoi(trimmed); err == nil {
		if format, ok := FormatByPID(pid); ok {
			return format, nil
		}
		return Format{PID: pid, Slug: fmt.Sprintf("format-%d", pid), Title: fmt.Sprintf("Format %d", pid), Status: StatusActive}, nil
	}
	for _, format := range Catalog {
		if strings.EqualFold(format.Title, trimmed) {
			return format, nil
		}
	}
	return Format{}, fmt.Errorf("unknown format: %s", value)
}

// FormatsByStatus groups the catalog by status. The order of the groups is
// the order of the statuses.
func FormatsByStatus() ([]string, map[string][]Format) {
	return GroupFormatsByStatus(Catalog)
}

// GroupFormatsByStatus groups a list of formats by status. The order of the
// groups is the order of the statuses.
func GroupFormatsByStatus(formats []Format) ([]string, map[string][]Format) {
	order := []string{StatusActive, StatusSeasonal, StatusArchived}
	grouped := map[string][]Format{}
	for _, status := range order {
		grouped[status] = []Format{}
	}
	for _, format := range formats {
		status := format.Status
		if _, ok := grouped[status]; !ok {
			grouped[status] = []Format{}
			order = append(order, status)
		}
		grouped[status] = append(grouped[status], format)
	}
	return order, grouped
}

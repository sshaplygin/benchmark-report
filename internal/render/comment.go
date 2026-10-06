package render

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sshaplygin/benchmark-report/internal/config"
	"github.com/sshaplygin/benchmark-report/internal/model"
)

const CommentByteBudget = 60000

var commentHeader = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,100}$`)

// Comment keeps complete reports intact and returns a publisher-sized Markdown
// body. artifactURL is needed only when the complete body exceeds the budget.
func Comment(comparison model.Comparison, cfg config.Config, header, artifactURL string) ([]byte, error) {
	if !commentHeader.MatchString(header) {
		return nil, fmt.Errorf("comment header must be 1 to 100 ASCII letters, digits, dots, underscores, colons, or hyphens")
	}
	presentation, full, err := Build(comparison, cfg)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(full) {
		return nil, fmt.Errorf("comment must contain valid UTF-8")
	}
	available := CommentByteBudget - len("\n<!-- Sticky Pull Request Comment"+header+" -->")
	if len(full) <= available {
		return full, nil
	}
	link, err := artifactLink(artifactURL)
	if err != nil {
		return nil, err
	}
	// Flattening and sorting restores the global order before grouping, so a
	// shortened grouped report retains the same selection prefix as the full one.
	rows := []model.PresentationRow{}
	for _, group := range presentation.Groups {
		rows = append(rows, group.Rows...)
	}
	sort.Slice(rows, func(i, j int) bool {
		if cfg.Report.Sort == "regression" && rank(rows[i].Signal) != rank(rows[j].Signal) {
			return rank(rows[i].Signal) < rank(rows[j].Signal)
		}
		return rows[i].Key < rows[j].Key
	})
	candidate := func(count int) []byte {
		p := presentation
		p.Disclosures = append([]string{}, presentation.Disclosures...)
		p.Sections.Benchstat = false
		if presentation.Sections.Benchstat {
			p.Disclosures = append(p.Disclosures, "Full benchstat details are available in the complete report artifact.")
		}
		if count < len(rows) && p.Sections.Tables {
			allowed := map[string]bool{}
			for _, row := range rows[:count] {
				allowed[row.Key] = true
			}
			p.Groups = []model.PresentationGroup{}
			for _, group := range presentation.Groups {
				kept := model.PresentationGroup{Labels: group.Labels, Rows: []model.PresentationRow{}}
				for _, row := range group.Rows {
					if allowed[row.Key] {
						kept.Rows = append(kept.Rows, row)
					}
				}
				if len(kept.Rows) > 0 {
					p.Groups = append(p.Groups, kept)
				}
			}
			p.Summary.Displayed = count
			p.Summary.Omitted = p.Summary.Selected - count
			configured := presentation.Summary.Omitted
			original := fmt.Sprintf("Showing %d of %d selected measurements; %d omitted by the configured row limit.", presentation.Summary.Displayed, presentation.Summary.Selected, configured)
			disclosures := []string{}
			for _, text := range p.Disclosures {
				if text != original {
					disclosures = append(disclosures, text)
				}
			}
			p.Disclosures = append(disclosures, fmt.Sprintf("Showing %d of %d selected measurements; %d omitted (%d by the configured row limit, %d by the comment byte budget).", count, p.Summary.Selected, p.Summary.Omitted, configured, len(rows)-count))
		}
		body := markdown(p, comparison)
		return append(body, []byte("\n[Complete report artifact](<"+link+">)\n")...)
	}
	maximum := len(rows)
	if !presentation.Sections.Tables {
		maximum = 0
	}
	completePrefix := candidate(maximum)
	if len(completePrefix) <= available {
		return completePrefix, nil
	}
	minimum := candidate(0)
	if len(minimum) > available {
		return nil, fmt.Errorf("comment fixed content exceeds the %d-byte budget including the upstream marker; shorten title, metadata, disclosures, or artifact URL", CommentByteBudget)
	}
	if maximum == 0 {
		return minimum, nil
	}
	// Each retained row adds a complete table row (and possibly group headings).
	// Binary search avoids re-rendering every possible prefix of large reports.
	low, high := 0, maximum-1
	for low < high {
		middle := low + (high-low+1)/2
		if len(candidate(middle)) <= available {
			low = middle
		} else {
			high = middle - 1
		}
	}
	result := candidate(low)
	if !utf8.Valid(result) || len(result) > available {
		return nil, fmt.Errorf("comment does not fit the UTF-8 byte budget")
	}
	return result, nil
}

func artifactLink(value string) (string, error) {
	invalid := func() (string, error) {
		return "", fmt.Errorf("an oversized comment requires an absolute HTTP or HTTPS artifact URL without credentials or control characters")
	}
	if !utf8.ValidString(value) {
		return invalid()
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return invalid()
		}
	}
	parsed, err := url.Parse(value)
	if err == nil {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
	}
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return invalid()
	}
	// Percent-encode delimiters that could terminate Markdown's angle destination.
	// Escape ampersands for Markdown entity decoding without changing query keys.
	var b strings.Builder
	for _, value := range []byte(parsed.String()) {
		if value <= 32 || value >= 127 || strings.ContainsRune("<>[]()\\`\"'", rune(value)) {
			fmt.Fprintf(&b, "%%%02X", value)
		} else if value == '&' {
			b.WriteString("&amp;")
		} else {
			b.WriteByte(value)
		}
	}
	return b.String(), nil
}

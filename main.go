package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	feedURL     = "https://mtil.uk/feed.xml"
	readmePath  = "README.md"
	startMarker = "<!-- BLOG:START -->"
	endMarker   = "<!-- BLOG:END -->"
	maxPosts    = 3
)

type RSS struct {
	Channel struct {
		Title   string  `xml:"title"`
		Entries []Entry `xml:"item"`
	} `xml:"channel"`
}

type Entry struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Published   string `xml:"pubDate"`
	Description string `xml:"description"`
}

func main() {
	resp, err := http.Get(feedURL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	items, err := mostRecentBlogs(body)
	if err != nil {
		panic(err)
	}

	if err := updateReadme(items); err != nil {
		panic(err)
	}
}

func mostRecentBlogs(body []byte) ([]Entry, error) {
	var feed RSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	items := feed.Channel.Entries
	sort.Slice(items, func(i, j int) bool {
		return parseDate(items[i].Published).After(parseDate(items[j].Published))
	})

	if len(items) > maxPosts {
		items = items[:maxPosts]
	}
	return items, nil
}

func parseDate(s string) time.Time {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

var (
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

func cleanDescription(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))

	const maxLen = 160
	if len(s) > maxLen {
		if cut := strings.LastIndex(s[:maxLen], " "); cut > 0 {
			s = s[:cut]
		} else {
			s = s[:maxLen]
		}
		s += "…"
	}
	return s
}

func updateReadme(items []Entry) error {
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}

	start := bytes.Index(content, []byte(startMarker))
	end := bytes.Index(content, []byte(endMarker))
	if start == -1 || end == -1 || end < start {
		return fmt.Errorf("markers %q / %q not found in %s", startMarker, endMarker, readmePath)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	for _, item := range items {
		date := ""
		if t := parseDate(item.Published); !t.IsZero() {
			date = t.Format("Jan 2006")
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s) — _%s_", item.Title, item.Link, date))
		if desc := cleanDescription(item.Description); desc != "" {
			sb.WriteString(fmt.Sprintf("<br>\n  %s", desc))
		}
		sb.WriteString("\n")
	}

	var updated []byte
	updated = append(updated, content[:start+len(startMarker)]...)
	updated = append(updated, sb.String()...)
	updated = append(updated, content[end:]...)

	return os.WriteFile(readmePath, updated, 0o644)
}

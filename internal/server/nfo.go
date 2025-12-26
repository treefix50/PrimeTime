package server

import (
	"encoding/xml"
	"os"
	"strings"
)

// NFO represents a minimal, parsed view of common Kodi-style .nfo files.
type NFO struct {
	Type        string   `json:"type"` // movie | tvshow | episode | unknown
	Title       string   `json:"title,omitempty"`
	Original    string   `json:"originalTitle,omitempty"`
	Plot        string   `json:"plot,omitempty"`
	Year        string   `json:"year,omitempty"`
	Rating      string   `json:"rating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Season      string   `json:"season,omitempty"`
	Episode     string   `json:"episode,omitempty"`
	ShowTitle   string   `json:"showTitle,omitempty"`
	RawRootName string   `json:"rawRoot"`
}

// ParseNFOFile parses a Kodi-style XML .nfo file.
// If the file does not exist or cannot be parsed, an error is returned.
func ParseNFOFile(path string) (*NFO, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	root := detectRootName(data)

	switch root {

	case "movie":
		var m struct {
			XMLName       xml.Name `xml:"movie"`
			Title         string   `xml:"title"`
			OriginalTitle string   `xml:"originaltitle"`
			Plot          string   `xml:"plot"`
			Year          string   `xml:"year"`
			Rating        string   `xml:"rating"`
			Genres        []string `xml:"genre"`
		}

		if err := xml.Unmarshal(data, &m); err != nil {
			return nil, err
		}

		return &NFO{
			Type:        "movie",
			Title:       strings.TrimSpace(m.Title),
			Original:    strings.TrimSpace(m.OriginalTitle),
			Plot:        strings.TrimSpace(m.Plot),
			Year:        strings.TrimSpace(m.Year),
			Rating:      strings.TrimSpace(m.Rating),
			Genres:      trimAll(m.Genres),
			RawRootName: root,
		}, nil

	case "tvshow":
		var t struct {
			XMLName xml.Name `xml:"tvshow"`
			Title   string   `xml:"title"`
			Plot    string   `xml:"plot"`
			Genres  []string `xml:"genre"`
		}

		if err := xml.Unmarshal(data, &t); err != nil {
			return nil, err
		}

		return &NFO{
			Type:        "tvshow",
			Title:       strings.TrimSpace(t.Title),
			Plot:        strings.TrimSpace(t.Plot),
			Genres:      trimAll(t.Genres),
			RawRootName: root,
		}, nil

	case "episodedetails":
		var e struct {
			XMLName   xml.Name `xml:"episodedetails"`
			Title     string   `xml:"title"`
			Plot      string   `xml:"plot"`
			Season    string   `xml:"season"`
			Episode   string   `xml:"episode"`
			ShowTitle string   `xml:"showtitle"`
			Rating    string   `xml:"rating"`
		}

		if err := xml.Unmarshal(data, &e); err != nil {
			return nil, err
		}

		return &NFO{
			Type:        "episode",
			Title:       strings.TrimSpace(e.Title),
			Plot:        strings.TrimSpace(e.Plot),
			Season:      strings.TrimSpace(e.Season),
			Episode:     strings.TrimSpace(e.Episode),
			ShowTitle:   strings.TrimSpace(e.ShowTitle),
			Rating:      strings.TrimSpace(e.Rating),
			RawRootName: root,
		}, nil

	default:
		// Unknown or unsupported root element
		return &NFO{
			Type:        "unknown",
			RawRootName: root,
		}, nil
	}
}

// detectRootName extracts the root XML element name from a .nfo file.
func detectRootName(data []byte) string {
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "\ufeff") {
		s = strings.TrimPrefix(s, "\ufeff")
		s = strings.TrimSpace(s)
	}

	// strip XML header
	if strings.HasPrefix(s, "<?xml") {
		if i := strings.Index(s, "?>"); i >= 0 {
			s = strings.TrimSpace(s[i+2:])
		}
	}

	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "<!--") {
			if i := strings.Index(s, "-->"); i >= 0 {
				s = s[i+3:]
				continue
			}
		}
		break
	}

	// expect <root ...>
	if strings.HasPrefix(s, "<") {
		s = s[1:]
	}

	end := strings.IndexAny(s, " >\n\r\t")
	if end == -1 {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(s[:end]))
}

// trimAll trims whitespace and removes empty strings.
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

package metadata

import (
	"strings"
)

func parseHTML(data []byte) Result {
	s := string(data)
	lower := strings.ToLower(s)
	var res Result

	// Extract <title>
	if idx := strings.Index(lower, "<title"); idx >= 0 {
		// find end of tag '>'
		gt := strings.Index(s[idx:], ">")
		if gt >= 0 {
			start := idx + gt + 1
			endTag := strings.Index(lower[start:], "</title")
			if endTag >= 0 {
				res.Title = strings.TrimSpace(stripTags(s[start : start+endTag]))
			}
		}
	}

	// Extract meta tags
	for i := 0; i < len(lower); {
		idx := strings.Index(lower[i:], "<meta")
		if idx < 0 {
			break
		}
		i += idx
		// find end of this tag
		gt := strings.Index(s[i:], ">")
		if gt < 0 {
			break
		}
		tagContent := s[i+5 : i+gt] // inside <meta ...>
		i += gt + 1
		attrs := parseAttrs(tagContent)
		name := strings.ToLower(strings.TrimSpace(attrs["name"]))
		prop := strings.ToLower(strings.TrimSpace(attrs["property"]))
		content := attrs["content"]
		if name == "description" && res.Description == "" {
			res.Description = strings.TrimSpace(content)
		}
		if prop == "og:title" && res.OGTitle == "" {
			res.OGTitle = strings.TrimSpace(content)
		}
		if prop == "og:description" && res.OGDescription == "" {
			res.OGDescription = strings.TrimSpace(content)
		}
		if prop == "og:image" && res.PreviewImageURL == "" {
			res.PreviewImageURL = strings.TrimSpace(content)
		}
	}

	// Extract favicon from <link ...>
	for i := 0; i < len(lower); {
		idx := strings.Index(lower[i:], "<link")
		if idx < 0 {
			break
		}
		i += idx
		gt := strings.Index(s[i:], ">")
		if gt < 0 {
			break
		}
		tagContent := s[i+5 : i+gt]
		i += gt + 1
		attrs := parseAttrs(tagContent)
		rel := strings.ToLower(strings.TrimSpace(attrs["rel"]))
		href := strings.TrimSpace(attrs["href"])
		if href == "" {
			continue
		}
		if strings.Contains(rel, "icon") {
			if res.FaviconURL == "" {
				res.FaviconURL = href
			}
		}
	}

	return res
}

func stripTags(s string) string {
	// very naive: remove tags
	var out strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// parseAttrs is a very simple attribute parser for key="value" or key='value'
func parseAttrs(s string) map[string]string {
	attrs := make(map[string]string)
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		// skip spaces
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		// read key
		eq := strings.Index(s, "=")
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(s[:eq])
		s = strings.TrimSpace(s[eq+1:])
		if len(s) == 0 {
			break
		}
		var val string
		if s[0] == '"' {
			s = s[1:]
			end := strings.Index(s, "\"")
			if end < 0 {
				break
			}
			val = s[:end]
			s = s[end+1:]
		} else if s[0] == '\'' {
			s = s[1:]
			end := strings.Index(s, "'")
			if end < 0 {
				break
			}
			val = s[:end]
			s = s[end+1:]
		} else {
			// unquoted
			end := strings.IndexAny(s, " >")
			if end < 0 {
				val = s
				s = ""
			} else {
				val = s[:end]
				s = s[end:]
			}
		}
		attrs[strings.ToLower(key)] = val
	}
	return attrs
}

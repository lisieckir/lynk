package metadata

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func parseHTML(data []byte) Result {
	// Work on a lowercase copy for tag/name/property matching, but extract
	// original bytes to preserve case/encoding as best we can.
	lower := bytes.ToLower(data)
	var res Result

	// Extract <title>...</title>
	if idx := bytes.Index(lower, []byte("<title")); idx >= 0 {
		gt := bytes.IndexByte(data[idx:], '>')
		if gt >= 0 {
			start := idx + gt + 1
			restLower := lower[start:]
			endTag := bytes.Index(restLower, []byte("</title"))
			if endTag >= 0 {
				raw := data[start : start+endTag]
				res.Title = strings.TrimSpace(stripTagsBytes(raw))
			}
		}
	}

	// Extract meta tags
	i := 0
	for i < len(lower) {
		idx := bytes.Index(lower[i:], []byte("<meta"))
		if idx < 0 {
			break
		}
		i += idx
		gt := bytes.IndexByte(data[i:], '>')
		if gt < 0 {
			break
		}
		tagContent := data[i+5 : i+gt] // inside <meta ...>
		i += gt + 1
		attrs := parseAttrsBytes(tagContent)
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
	i = 0
	for i < len(lower) {
		idx := bytes.Index(lower[i:], []byte("<link"))
		if idx < 0 {
			break
		}
		i += idx
		gt := bytes.IndexByte(data[i:], '>')
		if gt < 0 {
			break
		}
		tagContent := data[i+5 : i+gt]
		i += gt + 1
		attrs := parseAttrsBytes(tagContent)
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

func stripTagsBytes(b []byte) string {
	// naive strip, byte-safe
	var out strings.Builder
	inTag := false
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			// invalid utf8 byte, skip it safely
			b = b[1:]
			continue
		}
		if r == '<' {
			inTag = true
			b = b[size:]
			continue
		}
		if r == '>' {
			inTag = false
			b = b[size:]
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
		b = b[size:]
	}
	return out.String()
}

func parseAttrsBytes(s []byte) map[string]string {
	attrs := make(map[string]string)
	for len(s) > 0 {
		s = bytes.TrimSpace(s)
		if len(s) == 0 {
			break
		}
		eq := bytes.IndexByte(s, '=')
		if eq < 0 {
			break
		}
		key := strings.TrimSpace(string(s[:eq]))
		s = bytes.TrimSpace(s[eq+1:])
		if len(s) == 0 {
			break
		}
		var val string
		if s[0] == '"' {
			s = s[1:]
			end := bytes.IndexByte(s, '"')
			if end < 0 {
				break
			}
			val = string(s[:end])
			s = s[end+1:]
		} else if s[0] == '\'' {
			s = s[1:]
			end := bytes.IndexByte(s, '\'')
			if end < 0 {
				break
			}
			val = string(s[:end])
			s = s[end+1:]
		} else {
			// unquoted: read until space or >
			end := 0
			for end < len(s) {
				if s[end] == ' ' || s[end] == '>' || s[end] == '\t' || s[end] == '\n' || s[end] == '\r' {
					break
				}
				end++
			}
			if end == 0 {
				break
			}
			val = string(s[:end])
			s = s[end:]
		}
		attrs[strings.ToLower(key)] = val
	}
	return attrs
}

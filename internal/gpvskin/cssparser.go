package gpvskin

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// CSSRule holds a parsed CSS rule: selector + property map.
type CSSRule struct {
	Selector   string
	Properties map[string]string
}

// ParsedCSS is the full collection of rules from one CSS file.
type ParsedCSS struct {
	Rules   []CSSRule
	BaseURL string // base URL for resolving relative image URLs
}

// LoadCSS loads a CSS file from a URL or local path, strips comments, and parses rules.
func LoadCSS(source string) (*ParsedCSS, error) {
	var raw []byte
	var baseURL string

	if isURL(source) {
		resp, err := http.Get(source) //nolint:noctx
		if err != nil {
			return nil, fmt.Errorf("fetch CSS %s: %w", source, err)
		}
		defer resp.Body.Close()
		raw, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read CSS body: %w", err)
		}
		baseURL = source
	} else {
		var err error
		raw, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read CSS file %s: %w", source, err)
		}
		// Construct file:// base URL so relative image paths resolve correctly.
		abs, _ := os.Getwd()
		baseURL = "file://" + abs + "/" + source
	}

	text := string(raw)
	text = stripComments(text)
	rules := parseRules(text)

	return &ParsedCSS{Rules: rules, BaseURL: baseURL}, nil
}

// isURL returns true if s looks like an HTTP(S) URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// stripComments removes /* ... */ CSS comments.
func stripComments(css string) string {
	var sb strings.Builder
	i := 0
	for i < len(css) {
		if i+1 < len(css) && css[i] == '/' && css[i+1] == '*' {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 4
			continue
		}
		sb.WriteByte(css[i])
		i++
	}
	return sb.String()
}

// parseRules splits CSS text into individual rules.
func parseRules(css string) []CSSRule {
	var rules []CSSRule
	i := 0
	for i < len(css) {
		// Skip @-rules (import, media, keyframes, etc.)
		if j := strings.IndexByte(css[i:], '{'); j < 0 {
			break
		} else {
			selectorEnd := i + j
			selector := strings.TrimSpace(css[i:selectorEnd])
			// Find closing brace, handling nested {} (for @media)
			depth := 0
			bodyStart := selectorEnd + 1
			bodyEnd := bodyStart
			for bodyEnd < len(css) {
				if css[bodyEnd] == '{' {
					depth++
				} else if css[bodyEnd] == '}' {
					if depth == 0 {
						break
					}
					depth--
				}
				bodyEnd++
			}
			body := css[bodyStart:bodyEnd]
			i = bodyEnd + 1

			// Skip @-rules and nested blocks (they contain nested {})
			if strings.HasPrefix(selector, "@") || depth > 0 {
				// For @media blocks, recurse into the body to extract nested rules
				if strings.HasPrefix(selector, "@media") {
					nested := parseRules(body)
					rules = append(rules, nested...)
				}
				continue
			}

			// A selector can contain multiple comma-separated selectors
			for _, sel := range splitSelectors(selector) {
				sel = strings.TrimSpace(sel)
				if sel == "" {
					continue
				}
				props := parseProperties(body)
				if len(props) > 0 {
					rules = append(rules, CSSRule{Selector: sel, Properties: props})
				}
			}
		}
	}
	return rules
}

// splitSelectors splits a selector string by commas, but not inside brackets.
func splitSelectors(s string) []string {
	var result []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, s[start:i])
				start = i + 1
			}
		}
	}
	result = append(result, s[start:])
	return result
}

// parseProperties parses a CSS declaration block into a map.
func parseProperties(body string) map[string]string {
	props := make(map[string]string)
	for _, decl := range strings.Split(body, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		colon := strings.IndexByte(decl, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(decl[:colon]))
		val := strings.TrimSpace(decl[colon+1:])
		if key != "" && val != "" {
			props[key] = val
		}
	}
	return props
}

// --------------------------------------------------------------------------
// Selector matching helpers
// --------------------------------------------------------------------------

// MatchSelector tests whether a compound CSS selector string matches a given
// class-path. classpath is a slice of class-sets from outermost to innermost,
// e.g. [["controller","xbox"], ["trigger","left"]].
// We only implement the subset needed for GamepadViewer selectors.
func (pc *ParsedCSS) MatchSelector(sel string, classpath [][]string) bool {
	parts := strings.Fields(sel) // split on whitespace (descendant combinator)
	if len(parts) != len(classpath) {
		return false
	}
	for i, part := range parts {
		classes := classpath[i]
		if !selectorPartMatches(part, classes) {
			return false
		}
	}
	return true
}

// selectorPartMatches checks whether a single selector part (e.g. ".foo.bar")
// matches the given set of classes.
func selectorPartMatches(part string, classes []string) bool {
	// Split into class tokens (each starts with '.')
	tokens := strings.Split(part, ".")
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		found := false
		for _, c := range classes {
			if c == tok {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Lookup finds all CSS rules whose selector matches the given class-path and
// merges their properties (later rules override earlier ones).
// Special rule: a background or background-image value that contains no url()
// will NOT overwrite an existing url() value, to prevent bare keyword rules
// (e.g. `.xbox { background: no-repeat center }`) from erasing image URLs set
// by more specific rules earlier in the file.
func (pc *ParsedCSS) Lookup(classpath [][]string) map[string]string {
	merged := make(map[string]string)
	for _, rule := range pc.Rules {
		if pc.MatchSelector(rule.Selector, classpath) {
			for k, v := range rule.Properties {
				// For background / background-image, preserve existing URL.
				if k == "background" || k == "background-image" {
					if existing, ok := merged[k]; ok {
						existingHasURL := strings.Contains(existing, "url(")
						newHasURL := strings.Contains(v, "url(")
						if existingHasURL && !newHasURL {
							// New value has no URL — keep existing URL-bearing value.
							continue
						}
					}
				}
				merged[k] = v
			}
		}
	}
	return merged
}

// --------------------------------------------------------------------------
// Value parsing helpers
// --------------------------------------------------------------------------

var rePx = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)px$`)
var reNum = regexp.MustCompile(`^(-?\d+(?:\.\d+)?)$`)

// ParsePx parses a "123px" or plain integer value to int. Returns 0 on failure.
func ParsePx(s string) int {
	s = strings.TrimSpace(s)
	if m := rePx.FindStringSubmatch(s); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		return int(f)
	}
	if m := reNum.FindStringSubmatch(s); m != nil {
		f, _ := strconv.ParseFloat(m[1], 64)
		return int(f)
	}
	return 0
}

var reURL = regexp.MustCompile(`url\(["']?([^"')]+)["']?\)`)

// ParseBackgroundImage extracts the URL from a background or background-image value.
func ParseBackgroundImage(val string) string {
	m := reURL.FindStringSubmatch(val)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// ResolveURL resolves a (possibly relative) image URL against a base URL.
func ResolveURL(imageURL, base string) string {
	if imageURL == "" {
		return ""
	}
	if isURL(imageURL) {
		return imageURL
	}
	if strings.HasPrefix(imageURL, "file://") {
		return imageURL
	}
	// Resolve relative to base
	baseU, err := url.Parse(base)
	if err != nil {
		return imageURL
	}
	ref, err := url.Parse(imageURL)
	if err != nil {
		return imageURL
	}
	return baseU.ResolveReference(ref).String()
}

// ParseBackgroundPosition parses "X Y" background-position (px or named values).
// Returns (x, y) in pixels. Named values (left/center/right/top/bottom) return 0.
func ParseBackgroundPosition(val string) (int, int) {
	parts := strings.Fields(val)
	if len(parts) == 0 {
		return 0, 0
	}
	x := parseBGPosComponent(parts[0], false)
	y := 0
	if len(parts) >= 2 {
		y = parseBGPosComponent(parts[1], true)
	}
	return x, y
}

func parseBGPosComponent(s string, vertical bool) int {
	s = strings.TrimSpace(s)
	switch s {
	case "left", "top", "0%":
		return 0
	case "center", "50%":
		return -1 // sentinel for "center" — caller handles
	case "right", "bottom", "100%":
		return -2 // sentinel for "right/bottom"
	}
	return ParsePx(s)
}

// ParseBackground parses the CSS background shorthand and returns (imageURL, posX, posY).
func ParseBackground(val, base string) (string, int, int) {
	imgURL := ParseBackgroundImage(val)
	imgURL = ResolveURL(imgURL, base)

	// Remove url(...) from value to parse position
	cleaned := reURL.ReplaceAllString(val, "")
	// Remove color keywords and other noise
	cleaned = removeBackgroundKeywords(cleaned)
	posX, posY := ParseBackgroundPosition(strings.TrimSpace(cleaned))
	return imgURL, posX, posY
}

var bgKeywords = []string{
	"no-repeat", "repeat", "repeat-x", "repeat-y", "center",
	"scroll", "fixed", "local", "none", "transparent",
	"border-box", "padding-box", "content-box",
}

func removeBackgroundKeywords(s string) string {
	for _, kw := range bgKeywords {
		s = strings.ReplaceAll(s, kw, " ")
	}
	return s
}

// ParseTransform checks for rotateY(180deg) or similar horizontal mirror.
func ParseTransform(val string) bool {
	return strings.Contains(val, "rotateY(180deg)") || strings.Contains(val, "rotateY(-180deg)")
}

// ParseDisplay returns true if display:none.
func ParseDisplay(val string) bool {
	return strings.TrimSpace(strings.ToLower(val)) == "none"
}

// ParseFloat returns the float direction ("left", "right", or "").
func ParseFloat(val string) string {
	v := strings.TrimSpace(strings.ToLower(val))
	if v == "left" || v == "right" {
		return v
	}
	return ""
}

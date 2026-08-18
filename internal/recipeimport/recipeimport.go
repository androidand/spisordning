// Package recipeimport imports externally sourced recipes into review
// candidates. It finds the schema.org/Recipe JSON-LD node in a fetched recipe
// page, normalizes it into a source-agnostic ParsedRecipe, and splits the
// ingredient lines for canonicalization. It never writes to the household
// cookbook: an import produces a Candidate only, and promotion to cookbook
// content is a separate, explicit review action (see design.md, Section 4).
package recipeimport

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IngredientLine is one parsed ingredient line of an imported recipe — the
// SOURCE shape for external web recipes: RawText is always kept (the source
// line); Quantity/Unit are best-effort splits; Food is the derived food name
// (the line minus quantity+unit) used by the review surface. NeedsReview is
// always true at import: resolution to the canonical domain.Ingredient (id via
// domain.CanonicalIngredientID) happens in the review flow, reusing the
// ingredient_mapping.needs_review pattern (see design.md, Section 5.1). It is
// deliberately not the canonical type.
type IngredientLine struct {
	LineNo      int
	RawText     string
	Quantity    float64
	Unit        string
	Food        string
	NeedsReview bool
}

// ParsedRecipe is the normalized, source-agnostic result of parsing one
// external recipe. It mirrors the parsed-content columns of
// recipe_import_candidate. Times are whole seconds (schema.org ISO-8601
// durations are converted on parse). Provenance (source URL, external id,
// license note) lives on the Candidate, not here.
type ParsedRecipe struct {
	Title        string
	Description  string
	ImageURL     string
	Servings     int
	PrepSec      int
	CookSec      int
	TotalSec     int
	Category     string
	Cuisine      string
	Attribution  string
	Rating       float64
	RatingCount  int
	Nutrition    json.RawMessage
	Instructions []string
	Ingredients  []IngredientLine
	RawJSONLD    json.RawMessage
}

// Source is a registered external recipe source. It mirrors the
// external_recipe_source table.
type Source struct {
	ID          string
	Name        string
	Kind        string // jsonld_web | api | manual
	BaseURL     string
	LicenseNote string
	Decision    string // integrate_now | defer | omit
	Enabled     bool
}

// CandidateStatus is the lifecycle state of an import candidate.
type CandidateStatus string

const (
	// StatusCandidate is the state right after import: not yet in the cookbook.
	StatusCandidate CandidateStatus = "candidate"
	// StatusPromoted means the candidate was accepted into the cookbook and a
	// RecipeVariant was created for it.
	StatusPromoted CandidateStatus = "promoted"
	// StatusRejected means the candidate was reviewed and declined.
	StatusRejected CandidateStatus = "rejected"
)

// Candidate is a staged, not-yet-reviewed imported recipe with its provenance.
// It mirrors the recipe_import_candidate table. It is NOT cookbook content.
type Candidate struct {
	ID                int64
	SourceID          string
	SourceURL         string
	ExternalID        string
	LicenseNote       string
	ImportedAt        time.Time // set by the persistence layer
	FirstServedAt     time.Time // set when first planned/cooked
	Status            CandidateStatus
	PromotedVariantID string
	Parsed            ParsedRecipe
}

const ldJSONType = "application/ld+json"

// scriptBlocks returns the inner content of every
// <script type="application/ld+json"> block in the HTML, in document order. It
// scans by tag position rather than regex so it tolerates attribute order and
// quote style without a quote-sensitive pattern.
func scriptBlocks(html string) []string {
	var blocks []string
	rest := html
	for {
		start := strings.Index(rest, "<script")
		if start < 0 {
			break
		}
		gt := strings.IndexByte(rest[start:], '>')
		if gt < 0 {
			break
		}
		openTag := rest[start : start+gt+1]
		rest = rest[start+gt+1:]
		if !strings.Contains(strings.ToLower(openTag), ldJSONType) {
			continue
		}
		end := strings.Index(rest, "</script>")
		if end < 0 {
			break
		}
		blocks = append(blocks, rest[:end])
		rest = rest[end+len("</script>"):]
	}
	return blocks
}

// ExtractRecipeJSONLD finds the schema.org/Recipe node in an HTML document and
// returns its raw JSON. It scans every JSON-LD script block, decodes each, and
// returns the first node whose @type is "Recipe" -- handling a single
// top-level object, an array of nodes (Koket emits [Corporation, Recipe]), and
// an @graph wrapper. It returns an error when no Recipe node is present; that
// is the per-site-parser fallback trigger (see
// docs/research/recipe-web-import.md, 2.3).
func ExtractRecipeJSONLD(html string) (json.RawMessage, error) {
	for _, block := range scriptBlocks(html) {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var decoded any
		if err := json.Unmarshal([]byte(block), &decoded); err != nil {
			continue // not valid JSON; try the next block
		}
		if node, ok := findRecipeNode(decoded); ok {
			raw, err := json.Marshal(node)
			if err != nil {
				return nil, fmt.Errorf("recipeimport: re-encode recipe node: %w", err)
			}
			return raw, nil
		}
	}
	return nil, fmt.Errorf("recipeimport: no schema.org/Recipe JSON-LD node found")
}

// findRecipeNode searches a decoded JSON-LD value for an object whose @type is
// "Recipe" (alone or in a list), recursing through arrays and @graph wrappers.
func findRecipeNode(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if typeIsRecipe(t["@type"]) {
			return t, true
		}
		if graph, ok := t["@graph"]; ok {
			return findRecipeNode(graph)
		}
		return nil, false
	case []any:
		for _, e := range t {
			if node, ok := findRecipeNode(e); ok {
				return node, true
			}
		}
	}
	return nil, false
}

// typeIsRecipe reports whether a JSON-LD @type value (a string or a list of
// strings) includes "Recipe".
func typeIsRecipe(v any) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, "Recipe")
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.EqualFold(s, "Recipe") {
				return true
			}
		}
	}
	return false
}

// ParseRecipe maps a schema.org/Recipe JSON-LD node onto a ParsedRecipe.
func ParseRecipe(node json.RawMessage) (ParsedRecipe, error) {
	var m map[string]any
	if err := json.Unmarshal(node, &m); err != nil {
		return ParsedRecipe{}, fmt.Errorf("recipeimport: decode recipe node: %w", err)
	}
	var r ParsedRecipe
	r.Title = strField(m, "name")
	r.Description = strField(m, "description")
	r.ImageURL = firstString(m["image"])
	r.Servings = ParseYield(firstString(m["recipeYield"]))
	r.PrepSec = ParseDuration(strField(m, "prepTime"))
	r.CookSec = ParseDuration(strField(m, "cookTime"))
	r.TotalSec = ParseDuration(strField(m, "totalTime"))
	r.Category = firstString(m["recipeCategory"])
	r.Cuisine = firstString(m["recipeCuisine"])
	r.Attribution = authorName(m["author"])
	if ar, ok := m["aggregateRating"].(map[string]any); ok {
		r.Rating = numField(ar, "ratingValue")
		r.RatingCount = int(numField(ar, "ratingCount"))
		if r.RatingCount == 0 {
			r.RatingCount = int(numField(ar, "reviewCount"))
		}
	}
	if n, ok := m["nutrition"]; ok {
		if raw, err := json.Marshal(n); err == nil {
			r.Nutrition = raw
		}
	}
	r.Instructions = flattenInstructions(m["recipeInstructions"])
	if arr, ok := m["recipeIngredient"].([]any); ok {
		for _, e := range arr {
			line, ok := e.(string)
			if !ok || looksLikeNote(line) {
				continue
			}
			il := ParseIngredientLine(line)
			il.LineNo = len(r.Ingredients) + 1
			r.Ingredients = append(r.Ingredients, il)
		}
	}
	r.RawJSONLD = node
	return r, nil
}

// ParseIngredientLine splits one free-text ingredient line into quantity,
// unit, and food. It is deliberately conservative: when it cannot confidently
// split a line it keeps the whole line as the food text and flags it for
// review rather than guessing (see docs/research/recipe-web-import.md, 2.1).
func ParseIngredientLine(line string) IngredientLine {
	raw := strings.TrimSpace(line)
	il := IngredientLine{RawText: raw, NeedsReview: true}
	if raw == "" {
		return il
	}
	fields := strings.Fields(raw)
	if n, ok := parseNumber(fields[0]); ok {
		il.Quantity = n
		rest := fields[1:]
		if len(rest) >= 1 && isUnit(rest[0]) {
			il.Unit = rest[0]
			rest = rest[1:]
		}
		il.Food = strings.Join(rest, " ")
	}
	if il.Food == "" {
		il.Food = raw // no leading quantity: whole line is the food text
	}
	return il
}

// notePrefixes are Swedish lead-in words that mark a line as a section header
// or note rather than an ingredient (e.g. "Till en form (10 personer)").
var notePrefixes = []string{"till ", "för ", "notera", "obs ", "tips: ", "servera "}

// looksLikeNote reports whether a line is a section header or note rather than
// an ingredient. Conservative: only obvious note lead-ins are excluded;
// anything else is kept and left to the review flow.
func looksLikeNote(line string) bool {
	l := strings.ToLower(strings.TrimSpace(line))
	for _, p := range notePrefixes {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}

// knownUnits is the set of Swedish (and common metric) recipe units recognized
// when splitting an ingredient line.
var knownUnits = map[string]bool{
	"dl": true, "msk": true, "tsk": true, "förp": true, "st": true,
	"kg": true, "g": true, "kl": true, "klyfta": true, "klyftor": true,
	"l": true, "ml": true, "bit": true, "bitar": true,
}

func isUnit(s string) bool {
	return knownUnits[strings.ToLower(strings.TrimRight(s, ".,;"))]
}

func parseNumber(s string) (float64, bool) {
	s = strings.Replace(s, ",", ".", 1)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// ParseDuration converts an ISO-8601 duration (PT45M, PT1H30M, P1D) to whole
// seconds. Unparseable or empty input yields 0.
func ParseDuration(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 2 || s[0] != 'P' {
		return 0
	}
	total := 0
	var num int
	var hasNum bool
	flush := func(mult int) {
		if hasNum {
			total += num * mult
			num = 0
			hasNum = false
		}
	}
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			num = num*10 + int(c-'0')
			hasNum = true
		case c == 'D':
			flush(86400)
		case c == 'H':
			flush(3600)
		case c == 'M':
			flush(60)
		case c == 'S':
			flush(1)
		}
	}
	return total
}

// ParseYield extracts the first run of digits from a recipeYield value such as
// "10 portioner", "12 bitar", or "6".
func ParseYield(s string) int {
	n := 0
	seen := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			seen = true
		} else if seen {
			break
		}
	}
	if !seen {
		return 0
	}
	return n
}

// flattenInstructions walks a schema.org recipeInstructions value (a single
// HowToStep, a flat list of HowToStep, or nested HowToSection trees) and emits
// each step's text in document order.
func flattenInstructions(v any) []string {
	var out []string
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			if txt, ok := t["text"].(string); ok {
				out = append(out, txt)
			}
			if item, ok := t["itemListElement"]; ok {
				walk(item)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	return out
}

// CandidateFromParsed builds a review candidate from a parsed recipe plus its
// source and the fetched URL. This is Stage 7 (import): the result is a
// candidate, NOT cookbook content. Ingredient line numbers are assigned here.
func CandidateFromParsed(source Source, url string, parsed ParsedRecipe) Candidate {
	for i := range parsed.Ingredients {
		parsed.Ingredients[i].LineNo = i + 1
	}
	return Candidate{
		SourceID:    source.ID,
		SourceURL:   url,
		ExternalID:  TrailingID(url),
		LicenseNote: source.LicenseNote,
		Status:      StatusCandidate,
		Parsed:      parsed,
	}
}

// TrailingID returns the trailing run of digits in the last path segment of a
// URL (e.g. ".../potatisgratang-grundrecept-721833/" -> "721833"), or "" if
// there is none. It is a generic heuristic for a source's own recipe id.
func TrailingID(url string) string {
	u := strings.TrimRight(url, "/")
	seg := u
	if idx := strings.LastIndex(seg, "/"); idx >= 0 {
		seg = seg[idx+1:]
	}
	i := len(seg)
	for i > 0 && seg[i-1] >= '0' && seg[i-1] <= '9' {
		i--
	}
	return seg[i:]
}

// strField returns m[key] as a string, or "" if absent or not a string.
func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// firstString returns the first string in v, where v may be a string, a list
// of strings, or an object with a "url" field.
func firstString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				return s
			}
		}
	case map[string]any:
		if s, ok := t["url"].(string); ok {
			return s
		}
	}
	return ""
}

// numField returns m[key] as a float, parsing a numeric string if needed.
func numField(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// authorName returns the name of a schema.org author, which may be a string, a
// Person/Organization object, or a list of either.
func authorName(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["name"].(string); ok {
			return s
		}
	case []any:
		for _, e := range t {
			if s := authorName(e); s != "" {
				return s
			}
		}
	}
	return ""
}

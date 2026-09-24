package icons

import (
	"sort"
	"strings"
)

// Concepts maps a business concept to the bundled icon names that express it,
// best first.
//
// list_icons offered only a case-insensitive substring filter on the icon name,
// which misses business vocabulary entirely: "strategy", "revenue", "customer",
// "governance", "compliance", "innovation", "cost", "roadmap", "milestone" and
// "process" all returned zero hits, "risk" returned eight icons that merely
// contain "asterisk", and "team" returned "brand-teams" and "ironing-steam".
// Agents were left with icon-free grids or a multi-call guessing loop, and the
// Levenshtein suggestions that caught the guesses were nonsense ("strategy" →
// "karate", "revenue" → "venus") because edit distance over 5,000 glyph names
// cannot recover intent (go-slide-creator-3ojy).
//
// Every name here is asserted to exist in the outline set by
// TestConceptsResolveToRealIcons, so the map cannot drift from the shipped set.
var Concepts = map[string][]string{
	// --- Strategy and planning ---
	"strategy":       {"target", "chess", "map-2"},
	"goal":           {"target", "flag", "trophy"},
	"objective":      {"target", "flag"},
	"vision":         {"eye", "telescope", "binoculars"},
	"mission":        {"flag", "target"},
	"plan":           {"businessplan", "clipboard-list", "map-2"},
	"planning":       {"businessplan", "calendar", "clipboard-list"},
	"roadmap":        {"route", "map-2", "timeline"},
	"milestone":      {"flag", "map-pin", "calendar-event"},
	"phase":          {"stack-2", "layers-subtract"},
	"initiative":     {"rocket", "bulb"},
	"priority":       {"arrow-up", "star"},
	"scenario":       {"git-branch", "arrows-split"},
	"decision":       {"git-branch", "arrows-split", "help-circle"},
	"tradeoff":       {"scale", "arrows-left-right"},
	"benchmark":      {"chart-bar", "scale"},
	"opportunity":    {"bulb", "target", "trending-up"},
	"transformation": {"refresh", "arrows-shuffle", "recycle"},

	// --- Finance ---
	"revenue":    {"coin", "cash", "trending-up"},
	"sales":      {"coin", "shopping-cart", "trending-up"},
	"profit":     {"coin", "trending-up", "pig-money"},
	"margin":     {"percentage", "chart-pie"},
	"cost":       {"receipt", "coin", "trending-down"},
	"spend":      {"receipt", "wallet"},
	"budget":     {"wallet", "calculator", "pig-money"},
	"price":      {"tag", "receipt"},
	"pricing":    {"tag", "percentage"},
	"investment": {"pig-money", "trending-up", "building-bank"},
	"funding":    {"building-bank", "pig-money"},
	"cash":       {"cash", "coin", "wallet"},
	"cashflow":   {"cash", "arrows-exchange"},
	"savings":    {"pig-money", "wallet"},
	"tax":        {"receipt", "building-bank"},
	"finance":    {"building-bank", "coin", "calculator"},
	"valuation":  {"scale", "chart-line"},
	"forecast":   {"chart-line", "trending-up", "cloud"},

	// --- Customers and market ---
	"customer":     {"users", "user-check", "heart-handshake"},
	"client":       {"users", "briefcase"},
	"user":         {"user", "users"},
	"audience":     {"users-group", "speakerphone"},
	"segment":      {"chart-pie", "users-group"},
	"market":       {"world", "chart-bar", "building-store"},
	"competition":  {"chess", "trophy", "swords"},
	"competitor":   {"chess", "users"},
	"retention":    {"heart", "user-check", "refresh"},
	"churn":        {"user-minus", "trending-down"},
	"loyalty":      {"heart", "award"},
	"satisfaction": {"heart", "star", "mood-smile"},
	"feedback":     {"message-2", "star"},
	"brand":        {"award", "star"},
	"marketing":    {"speakerphone", "target"},
	"partnership":  {"heart-handshake", "users"},
	"stakeholder":  {"users-group", "user-check"},

	// --- People and organisation ---
	"team":         {"users", "users-group"},
	"people":       {"users", "user"},
	"talent":       {"user-star", "school"},
	"hiring":       {"user-plus", "briefcase"},
	"recruitment":  {"user-plus", "briefcase"},
	"training":     {"school", "certificate"},
	"skill":        {"school", "award"},
	"leadership":   {"user-star", "flag"},
	"culture":      {"heart", "users-group"},
	"organisation": {"sitemap", "hierarchy", "building"},
	"organization": {"sitemap", "hierarchy", "building"},
	"structure":    {"sitemap", "hierarchy", "topology-star"},
	"headcount":    {"users", "user"},
	"role":         {"user-check", "briefcase"},

	// --- Operations and process ---
	"process":       {"route", "arrows-shuffle", "settings"},
	"workflow":      {"route", "git-branch", "arrows-shuffle"},
	"operation":     {"settings", "tool"},
	"efficiency":    {"gauge", "bolt", "trending-up"},
	"productivity":  {"gauge", "trending-up"},
	"automation":    {"robot", "settings", "bolt"},
	"quality":       {"award", "checklist", "shield-check"},
	"capacity":      {"gauge", "stack-2"},
	"throughput":    {"gauge", "arrows-right"},
	"supply":        {"truck", "package"},
	"logistics":     {"truck", "package", "route"},
	"inventory":     {"package", "stack-2", "building-warehouse"},
	"manufacturing": {"building-factory", "settings"},
	"maintenance":   {"tool", "settings", "refresh"},
	"scale":         {"trending-up", "stack-2"},
	"speed":         {"bolt", "gauge", "rocket"},
	"delivery":      {"truck", "package", "checklist"},

	// --- Risk, governance, compliance ---
	"risk":          {"alert-triangle", "shield-exclamation", "alert-circle"},
	"issue":         {"alert-circle", "bug"},
	"threat":        {"alert-triangle", "shield-x"},
	"mitigation":    {"shield-check", "tool"},
	"governance":    {"scale", "building-bank", "gavel"},
	"compliance":    {"certificate", "shield-check", "clipboard-check"},
	"regulation":    {"gavel", "scale", "file-certificate"},
	"audit":         {"clipboard-check", "search", "file-search"},
	"policy":        {"file-certificate", "clipboard-list"},
	"control":       {"adjustments", "shield-check"},
	"security":      {"shield-lock", "lock", "key"},
	"privacy":       {"lock", "eye-off", "shield-lock"},
	"legal":         {"gavel", "scale", "file-certificate"},
	"contract":      {"file-certificate", "signature"},
	"certification": {"certificate", "award"},

	// --- Technology ---
	"technology":      {"cpu", "device-desktop", "server"},
	"innovation":      {"bulb", "rocket", "flask"},
	"research":        {"flask", "microscope", "search"},
	"data":            {"database", "chart-bar", "table"},
	"analytics":       {"chart-line", "chart-bar", "chart-dots"},
	"platform":        {"server", "stack-2", "cloud"},
	"infrastructure":  {"server", "building", "network"},
	"cloud":           {"cloud", "server"},
	"software":        {"code", "device-desktop"},
	"integration":     {"plug-connected", "git-merge", "network"},
	"api":             {"plug-connected", "code"},
	"network":         {"network", "topology-star", "world"},
	"digital":         {"device-mobile", "cpu"},
	"ai":              {"brain", "cpu", "robot"},
	"automation-tech": {"robot", "cpu"},
	"migration":       {"arrows-transfer-up", "server", "cloud-upload"},
	"architecture":    {"topology-star", "sitemap", "building"},

	// --- Measurement and reporting ---
	"growth":       {"trending-up", "chart-line", "rocket"},
	"decline":      {"trending-down", "chart-line"},
	"performance":  {"gauge", "chart-line", "trophy"},
	"metric":       {"gauge", "chart-bar"},
	"kpi":          {"gauge", "chart-bar", "target"},
	"report":       {"file-analytics", "chart-bar", "presentation"},
	"dashboard":    {"layout-dashboard", "gauge"},
	"trend":        {"trending-up", "chart-line"},
	"comparison":   {"scale", "chart-bar", "arrows-left-right"},
	"share":        {"chart-pie", "percentage"},
	"target-state": {"target", "flag"},
	"baseline":     {"ruler", "chart-bar"},

	// --- Time ---
	"time":     {"clock", "hourglass"},
	"schedule": {"calendar", "clock"},
	"deadline": {"calendar-event", "hourglass", "alert-circle"},
	"timeline": {"timeline", "calendar", "route"},
	"quarter":  {"calendar", "chart-bar"},
	"history":  {"history", "clock"},

	// --- Communication ---
	"communication": {"message-2", "speakerphone", "mail"},
	"meeting":       {"users", "calendar", "presentation"},
	"presentation":  {"presentation", "chart-bar"},
	"document":      {"file-text", "clipboard-list"},
	"email":         {"mail", "send"},
	"support":       {"headset", "lifebuoy", "message-2"},
	"collaboration": {"users", "heart-handshake", "git-merge"},
	"announcement":  {"speakerphone", "bell"},
}

// ConceptMatch is one icon suggested for a concept query.
type ConceptMatch struct {
	// Concept is the matched concept key.
	Concept string
	// Name is the bundled icon name (bare, outline set).
	Name string
	// Rank is the icon's position in the concept's preference list (0 = best).
	Rank int
}

// MatchConcepts returns the icons mapped to a business concept, best first.
//
// The query matches a concept key exactly (case- and space-insensitive), or as a
// whole word inside one — so "cost reduction" reaches "cost" and "customer
// retention" reaches both "customer" and "retention". Results keep each
// concept's own preference order and are deduplicated by icon name.
func MatchConcepts(query string) []ConceptMatch {
	q := normalizeConcept(query)
	if q == "" {
		return nil
	}

	type hit struct {
		concept string
		name    string
		rank    int
		exact   bool
	}
	var hits []hit
	for concept, names := range Concepts {
		norm := normalizeConcept(concept)
		exact := norm == q
		if !exact && !conceptWordMatch(q, norm) {
			continue
		}
		for i, n := range names {
			hits = append(hits, hit{concept: concept, name: n, rank: i, exact: exact})
		}
	}

	// Exact concept matches first, then by the icon's rank within its concept,
	// then alphabetically so the result is deterministic.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].exact != hits[j].exact {
			return hits[i].exact
		}
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		if hits[i].concept != hits[j].concept {
			return hits[i].concept < hits[j].concept
		}
		return hits[i].name < hits[j].name
	})

	seen := make(map[string]bool, len(hits))
	out := make([]ConceptMatch, 0, len(hits))
	for _, h := range hits {
		if seen[h.name] {
			continue
		}
		seen[h.name] = true
		out = append(out, ConceptMatch{Concept: h.concept, Name: h.name, Rank: h.rank})
	}
	return out
}

// ConceptNames returns just the icon names MatchConcepts found, best first.
func ConceptNames(query string) []string {
	matches := MatchConcepts(query)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m.Name)
	}
	return names
}

// ConceptKeys returns every concept key, sorted. Agents use it to see the
// vocabulary the concept index covers.
func ConceptKeys() []string {
	keys := make([]string, 0, len(Concepts))
	for k := range Concepts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// normalizeConcept lower-cases a concept or query and collapses separators to
// single spaces, so "Cost-Reduction" and "cost reduction" compare equal.
func normalizeConcept(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("-", " ", "_", " ", "/", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// conceptWordMatch reports whether concept appears as a whole word in query, or
// query as a whole word in concept. Whole-word matching is what keeps "risk"
// from reaching "asterisk".
func conceptWordMatch(query, concept string) bool {
	return containsWord(query, concept) || containsWord(concept, query)
}

// containsWord reports whether needle appears in haystack as a complete
// space-delimited word sequence.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	hayWords := strings.Fields(haystack)
	needleWords := strings.Fields(needle)
	if len(needleWords) == 0 || len(needleWords) > len(hayWords) {
		return false
	}
	for i := 0; i+len(needleWords) <= len(hayWords); i++ {
		match := true
		for j, w := range needleWords {
			if hayWords[i+j] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

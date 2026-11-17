package aixm45to51

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/davidkohl/goaixm/convert"
)

// The mapping tables: rules (mapping.tsv) and code lists (codes.tsv).  They
// are the reviewable statement of the conversion; the code refers to rules
// by id, and ConverterID.MappingDigest is computed over both files.
var (
	//go:embed mapping.tsv
	mappingTSV []byte
	//go:embed codes.tsv
	codesTSV []byte
)

// Rule is one row of the mapping table.
type Rule struct {
	ID, Source, Target string
	Outcome            convert.Outcome
	// Ref names the mapping document section the rule follows; "goaixm"
	// marks a goaixm proposal not yet reconciled with the official mapping.
	Ref, Note string
}

// CodeList is one list of the code table.
type CodeList struct {
	Name string
	// Type is the 5.1.1 simple type of the values.
	Type string
	// Other reports whether an unknown value becomes OTHER (with a note);
	// false: it is invalid and dropped.
	Other bool
	// Codes maps 4.5 values to 5.1.1 values.
	Codes map[string]string
	// Order lists the 4.5 values in table order.
	Order []string
}

var (
	rules     map[string]Rule
	ruleOrder []string
	codeLists map[string]*CodeList
)

func init() {
	var err error
	if rules, ruleOrder, err = parseRules(mappingTSV); err != nil {
		panic(err)
	}
	if codeLists, err = parseCodes(codesTSV); err != nil {
		panic(err)
	}
}

func lines(b []byte) [][]string {
	var out [][]string
	for _, l := range strings.Split(string(b), "\n") {
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, strings.Split(l, "\t"))
	}
	return out
}

func parseRules(b []byte) (map[string]Rule, []string, error) {
	m := map[string]Rule{}
	var order []string
	for _, f := range lines(b) {
		if len(f) < 5 || len(f) > 6 {
			return nil, nil, fmt.Errorf("aixm45to51: mapping.tsv: bad line %q", strings.Join(f, "\t"))
		}
		r := Rule{ID: f[0], Source: f[1], Target: f[2], Outcome: convert.Outcome(f[3]), Ref: f[4]}
		if len(f) == 6 {
			r.Note = f[5]
		}
		valid := false
		for _, o := range convert.Outcomes {
			valid = valid || o == r.Outcome
		}
		if !valid {
			return nil, nil, fmt.Errorf("aixm45to51: mapping.tsv: rule %s: outcome %q", r.ID, r.Outcome)
		}
		if _, dup := m[r.ID]; dup {
			return nil, nil, fmt.Errorf("aixm45to51: mapping.tsv: duplicate rule %s", r.ID)
		}
		m[r.ID] = r
		order = append(order, r.ID)
	}
	return m, order, nil
}

func parseCodes(b []byte) (map[string]*CodeList, error) {
	m := map[string]*CodeList{}
	for _, f := range lines(b) {
		switch {
		case f[0] == "list" && len(f) == 4 && (f[3] == "other" || f[3] == "closed"):
			if m[f[1]] != nil {
				return nil, fmt.Errorf("aixm45to51: codes.tsv: duplicate list %s", f[1])
			}
			m[f[1]] = &CodeList{Name: f[1], Type: f[2], Other: f[3] == "other", Codes: map[string]string{}}
		case f[0] == "code" && len(f) == 5:
			l := m[f[1]]
			if l == nil {
				return nil, fmt.Errorf("aixm45to51: codes.tsv: code of undeclared list %s", f[1])
			}
			if _, dup := l.Codes[f[2]]; dup {
				return nil, fmt.Errorf("aixm45to51: codes.tsv: duplicate code %s/%s", f[1], f[2])
			}
			l.Codes[f[2]] = f[3]
			l.Order = append(l.Order, f[2])
		default:
			return nil, fmt.Errorf("aixm45to51: codes.tsv: bad line %q", strings.Join(f, "\t"))
		}
	}
	return m, nil
}

// Rules returns the mapping rules in table order.
func Rules() []Rule {
	out := make([]Rule, len(ruleOrder))
	for i, id := range ruleOrder {
		out[i] = rules[id]
	}
	return out
}

// CodeLists returns the code lists by name (copies).
func CodeLists() map[string]CodeList {
	out := make(map[string]CodeList, len(codeLists))
	for n, l := range codeLists {
		c := *l
		c.Codes = make(map[string]string, len(l.Codes))
		for k, v := range l.Codes {
			c.Codes[k] = v
		}
		c.Order = append([]string(nil), l.Order...)
		out[n] = c
	}
	return out
}

func rule(id string) Rule {
	r, ok := rules[id]
	if !ok {
		panic("aixm45to51: unknown rule " + id)
	}
	return r
}

func list(name string) *CodeList {
	l, ok := codeLists[name]
	if !ok {
		panic("aixm45to51: unknown code list " + name)
	}
	return l
}

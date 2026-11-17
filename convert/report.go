package convert

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Outcome says what happened to one source value.
type Outcome string

// The outcomes of a source leaf.
const (
	Mapped    Outcome = "mapped"    // value carried over (possibly reformatted)
	Derived   Outcome = "derived"   // computed (angles, merged features, identifiers)
	Defaulted Outcome = "defaulted" // target value supplied without a source value
	Lossy     Outcome = "lossy"     // carried with loss (approximated geometry, free text to OTHER)
	Unmapped  Outcome = "unmapped"  // source value not carried
	Invalid   Outcome = "invalid"   // source value unusable (bad coordinate, unknown code)
)

// Outcomes lists every Outcome in a fixed order.
var Outcomes = []Outcome{Mapped, Derived, Defaulted, Lossy, Unmapped, Invalid}

// Severity of an Issue.
type Severity string

// Issue severities.
const (
	Info    Severity = "info"
	Warning Severity = "warning"
	Error   Severity = "error"
)

// ReportFormat is the version of the report layout, stored in
// Report.Format.
const ReportFormat = 1

// MaxSamples is the number of samples kept per element path.
const MaxSamples = 5

// MaxIssuesPerCode is the number of distinct issues kept per code; further
// occurrences are folded into one overflow entry, so counts stay exact.
const MaxIssuesPerCode = 100

// Report accounts for a conversion.  Converters fill Kinds and Issues; Run
// fills the rest.  It contains no timings, so equal input and ConverterID
// give an equal report and Digest.
type Report struct {
	// Format is the report layout version (ReportFormat).
	Format int `json:"format"`
	// Converter identifies the rules used.
	Converter ConverterID `json:"converter"`
	// From and To are the source and target versions.
	From version.Version `json:"from"`
	To   version.Version `json:"to"`
	// Source describes the input document.
	Source SourceInfo `json:"source"`
	// Output describes the converted document.
	Output OutputInfo `json:"output"`
	// Kinds holds the statistics per source kind (e.g. "Ase").
	Kinds map[string]*KindStats `json:"kinds"`
	// Targets counts the records per target kind.
	Targets map[string]int `json:"targets"`
	// Issues lists problems with the source data, sorted, capped per code
	// (see MaxIssuesPerCode).
	Issues []Issue `json:"issues"`
	// Steps holds the reports of the converters of a Chain.
	Steps []*Report `json:"steps,omitempty"`

	issueIdx map[issueKey]int
	perCode  map[string]int
}

// SourceInfo describes the input document of a conversion.
type SourceInfo struct {
	Name    string            `json:"name"`
	Bytes   int64             `json:"bytes"`
	SHA256  string            `json:"sha256"`
	Version version.Version   `json:"version"`
	Root    string            `json:"root"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

// OutputInfo describes the converted document: the number of records and
// the size and sha256 of its XMLWriter form (for targets before 5.x, of the
// concatenated record XML).
type OutputInfo struct {
	Records int    `json:"records"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
}

// KindStats accounts for the source elements of one kind.
//
// Invariants (Report.Check): In == Out + Merged + Dropped, and Leaves equals
// the sum of all element outcomes.
type KindStats struct {
	// In counts the source elements of this kind.
	In int `json:"in"`
	// Out counts those emitted as (the primary source of) a record.
	Out int `json:"out"`
	// Merged counts those folded into a record of another element.
	Merged int `json:"merged"`
	// Dropped counts those not carried at all.
	Dropped int `json:"dropped"`
	// Leaves counts the source leaves visited (see WalkLeaves).
	Leaves int `json:"leaves"`
	// Elements holds the outcomes by source path, e.g. "Ase/valDistVerUpper"
	// or "Avx[codeType=RHL]".
	Elements map[string]*ElementStats `json:"elements"`
}

// ElementStats accounts for the values found at one source path.
type ElementStats struct {
	// Target is the target path, if any.
	Target string `json:"target,omitempty"`
	// Rule is the id of the mapping rule applied.
	Rule string `json:"rule"`
	// Outcomes counts the values by outcome.
	Outcomes map[Outcome]int `json:"outcomes"`
	// Samples holds the first MaxSamples recorded examples.
	Samples []Sample `json:"samples,omitempty"`
}

// Sample is an example value for a report entry.
type Sample struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
}

// Issue is a problem with the source data (dangling reference, duplicate
// key, unknown datum, …).
type Issue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Kind     string   `json:"kind,omitempty"`
	Key      string   `json:"key,omitempty"`
	Message  string   `json:"message,omitempty"`
	// Count is the number of occurrences folded into this entry.
	Count int `json:"count"`
}

type issueKey struct {
	code, kind, key, msg string
	sev                  Severity
}

// NewReport returns an empty report for a converter to fill.
func NewReport() *Report {
	return &Report{Kinds: map[string]*KindStats{}, Targets: map[string]int{}}
}

// Kind returns the statistics of a source kind, creating them.
func (r *Report) Kind(name string) *KindStats {
	if r.Kinds == nil {
		r.Kinds = map[string]*KindStats{}
	}
	k := r.Kinds[name]
	if k == nil {
		k = &KindStats{Elements: map[string]*ElementStats{}}
		r.Kinds[name] = k
	}
	return k
}

// Record adds one outcome at a source path.  rule and target are kept from
// the first call for a path.  sample, when not nil, is kept while fewer than
// MaxSamples are stored.
func (k *KindStats) Record(path string, o Outcome, rule, target string, sample *Sample) {
	if k.Elements == nil {
		k.Elements = map[string]*ElementStats{}
	}
	e := k.Elements[path]
	if e == nil {
		e = &ElementStats{Rule: rule, Target: target, Outcomes: map[Outcome]int{}}
		k.Elements[path] = e
	}
	e.Outcomes[o]++
	if sample != nil && len(e.Samples) < MaxSamples {
		e.Samples = append(e.Samples, *sample)
	}
}

// AddIssue records an issue.  Equal issues (code, severity, kind, key,
// message) are folded into one entry; beyond MaxIssuesPerCode distinct
// entries per code, occurrences go into one overflow entry of that code.
// A zero Count counts as 1.
func (r *Report) AddIssue(i Issue) {
	if i.Count <= 0 {
		i.Count = 1
	}
	if r.issueIdx == nil {
		r.issueIdx = map[issueKey]int{}
		r.perCode = map[string]int{}
		for j, e := range r.Issues {
			r.issueIdx[issueKey{e.Code, e.Kind, e.Key, e.Message, e.Severity}] = j
			r.perCode[e.Code]++
		}
	}
	k := issueKey{i.Code, i.Kind, i.Key, i.Message, i.Severity}
	if j, ok := r.issueIdx[k]; ok {
		r.Issues[j].Count += i.Count
		return
	}
	if r.perCode[i.Code] >= MaxIssuesPerCode {
		i = Issue{Code: i.Code, Severity: i.Severity, Message: "further occurrences", Count: i.Count}
		k = issueKey{code: i.Code, msg: i.Message, sev: i.Severity}
		if j, ok := r.issueIdx[k]; ok {
			r.Issues[j].Count += i.Count
			return
		}
	}
	r.issueIdx[k] = len(r.Issues)
	r.perCode[i.Code]++
	r.Issues = append(r.Issues, i)
}

func (r *Report) sortIssues() {
	sort.SliceStable(r.Issues, func(a, b int) bool {
		x, y := r.Issues[a], r.Issues[b]
		if x.Code != y.Code {
			return x.Code < y.Code
		}
		if x.Severity != y.Severity {
			return x.Severity < y.Severity
		}
		if x.Kind != y.Kind {
			return x.Kind < y.Kind
		}
		if x.Key != y.Key {
			return x.Key < y.Key
		}
		return x.Message < y.Message
	})
	r.issueIdx, r.perCode = nil, nil
}

// Count returns the number of values with outcome o, over all kinds and
// chain steps.
func (r *Report) Count(o Outcome) int {
	n := 0
	for _, k := range r.Kinds {
		for _, e := range k.Elements {
			n += e.Outcomes[o]
		}
	}
	for _, s := range r.Steps {
		n += s.Count(o)
	}
	return n
}

// IssueCount returns the number of issue occurrences of severity sev ("" for
// all), over all chain steps.
func (r *Report) IssueCount(sev Severity) int {
	n := 0
	for _, i := range r.Issues {
		if sev == "" || i.Severity == sev {
			n += i.Count
		}
	}
	for _, s := range r.Steps {
		n += s.IssueCount(sev)
	}
	return n
}

// ErrAccounting is returned by Check (and Run) when a report's numbers do
// not add up: a converter bug, never a data problem.
var ErrAccounting = errors.New("convert: report accounting violated")

// Check verifies the accounting of the report and its steps: for every
// kind In == Out + Merged + Dropped and Leaves == sum of outcomes; the
// output record count equals the sum of Targets.
func (r *Report) Check() error {
	var errs []error
	names := make([]string, 0, len(r.Kinds))
	for n := range r.Kinds {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		k := r.Kinds[n]
		if k.In != k.Out+k.Merged+k.Dropped {
			errs = append(errs, fmt.Errorf("%w: kind %s: in %d != out %d + merged %d + dropped %d", ErrAccounting, n, k.In, k.Out, k.Merged, k.Dropped))
		}
		sum := 0
		for _, e := range k.Elements {
			for _, c := range e.Outcomes {
				sum += c
			}
		}
		if sum != k.Leaves {
			errs = append(errs, fmt.Errorf("%w: kind %s: %d leaves, %d outcomes", ErrAccounting, n, k.Leaves, sum))
		}
	}
	t := 0
	for _, c := range r.Targets {
		t += c
	}
	if t != r.Output.Records {
		errs = append(errs, fmt.Errorf("%w: %d records, targets sum to %d", ErrAccounting, r.Output.Records, t))
	}
	for i, s := range r.Steps {
		if err := s.Check(); err != nil {
			errs = append(errs, fmt.Errorf("step %d: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

// JSON returns the canonical JSON form of the report: fixed field order,
// sorted map keys, no timings.
func (r *Report) JSON() ([]byte, error) { return json.Marshal(r) }

// Digest returns the sha256 (hex) of the canonical JSON form.
func (r *Report) Digest() string {
	b, err := r.JSON()
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Leaf is one source value found by WalkLeaves.
type Leaf struct {
	// Path is the element path from the walked element, local names joined
	// by "/", with "/@name" for an attribute: "Ase/AseUid/codeId",
	// "Ase/@mid".
	Path string
	// Value is the attribute value or the element's text.
	Value string
	// Node is the element holding the value.
	Node *xmlnode.Node
	// Attr is the attribute's local name, "" for element text.
	Attr string
}

// WalkLeaves calls fn for every leaf of the tree rooted at n, in document
// order: every attribute, and every element without child elements (with
// its text, possibly empty).  Text beside child elements (mixed content)
// is not a leaf.  These are the values a converter accounts
// for; CountLeaves gives their number.
func WalkLeaves(n *xmlnode.Node, fn func(Leaf)) {
	walkLeaves(n, n.Local(), fn)
}

func walkLeaves(n *xmlnode.Node, path string, fn func(Leaf)) {
	for _, a := range n.Attrs {
		fn(Leaf{Path: path + "/@" + a.Name.Local, Value: a.Value, Node: n, Attr: a.Name.Local})
	}
	if len(n.Children) == 0 {
		fn(Leaf{Path: path, Value: n.Text, Node: n})
		return
	}
	for _, c := range n.Children {
		walkLeaves(c, path+"/"+c.Local(), fn)
	}
}

// CountLeaves returns the number of leaves WalkLeaves visits.
func CountLeaves(n *xmlnode.Node) int {
	c := 0
	WalkLeaves(n, func(Leaf) { c++ })
	return c
}

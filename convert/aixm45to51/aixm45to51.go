// Package aixm45to51 converts AIXM 4.5 snapshots to AIXM 5.1.1.
//
// It implements convert.Converter: the output is one standalone 5.1.1
// feature element (convert.Record) per converted entity, in document order,
// so goaixm.Options.Converters can decode a 4.5 file like a native 5.1.1
// message and convert.XMLWriter can write the converted document.
//
// Rules live in two embedded tables, mapping.tsv (rules by id: source path,
// target path, nominal outcome, reference) and codes.tsv (code and unit
// lists).  Both are goaixm's own proposal until reconciled with the
// official EUROCONTROL AICM/AIXM 4.5 to AIXM 5.1 mapping; their reference
// column says so per rule.  ConverterID.MappingDigest hashes them.
//
// Identity: each feature gets a UUIDv5 over "aixm45:" and the canonical
// natural key of its …Uid (aixm45.Key) in the caller's Config.Namespace;
// references are computed from the referencing …Uid the same way, so they
// need no lookup.  A natural key that occurs more than once keeps its key
// for the first occurrence; later ones get "#2", "#3", … and a
// duplicate_key warning.
//
// Time: every feature has one BASELINE time slice whose validTime and
// featureLifetime begin at the snapshot's effective attribute (verbatim,
// with its offset) and end indeterminately; sequenceNumber Config.Sequence
// (default 1), correctionNumber 0.
//
// Positions are written as decimal degrees (lat long, EPSG:4326) with
// enough digits to reproduce the source.  Coordinates on a datum other than
// WGS-84 (codeDatum other than WGE, e.g. U for unknown) are carried
// untransformed, flagged with a WARNING note on the feature and reported as
// an error issue per feature.
//
// Coverage (this version): Org, Uni, Dpn, Vor, Dme, Ndb, Tcn (with the
// derived Navaid), Obs, Ahp, Rwy, Rdn.  Airspaces and borders (Ase, Abd,
// Adg, Gbr, …) and every other kind are reported per leaf as unmapped
// (rules PH1B and LATER) and dropped.  AIXM-Update documents are refused.
//
// The report accounts for every source leaf: each attribute and each
// element text of every top-level element (and of the root) gets exactly
// one outcome; leaves no rule names are unmapped (rule UNMAPPED).
package aixm45to51

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Name and Version identify the rules (convert.ConverterID).  Version is
// bumped on every rule change.
const (
	Name    = "goaixm/aixm45to51"
	Version = "0.1.0"
)

// Config configures a Converter.
type Config struct {
	// Namespace is the identity namespace of the data originator (required):
	// UUIDs are UUIDv5(Namespace, "aixm45:" + natural key).  Use one fixed
	// namespace per producer and never change it; changing it gives every
	// feature a new identity.
	Namespace convert.UUID
	// Sequence, when set, gives the sequenceNumber of the time slices from
	// the document header; default 1.
	Sequence func(aixm45.Header) uint32
	// ObstacleType, when set, maps an obstacle's free-text type
	// (Obs/txtDescrType) to a 5.1.1 CodeVerticalStructureType value; "" (or
	// a value that is not in the list) keeps the default: OTHER with the
	// text in a note.  goaixm ships no such table: the texts are the
	// producer's own.
	ObstacleType func(descr string) string
}

// Errors of Convert.
var (
	ErrNoNamespace = errors.New("aixm45to51: Config.Namespace is required")
	ErrUpdate      = errors.New("aixm45to51: AIXM-Update documents are not supported")
	ErrNoEffective = errors.New("aixm45to51: snapshot without a valid effective date")
)

// Converter converts AIXM 4.5 snapshots to AIXM 5.1.1.
type Converter struct {
	cfg Config
	id  convert.ConverterID
}

// New returns a converter.  Convert fails with ErrNoNamespace when
// cfg.Namespace is the nil UUID.
func New(cfg Config) *Converter {
	return &Converter{cfg: cfg, id: convert.NewID(Name, Version, mappingTSV, codesTSV)}
}

// From implements convert.Converter: AIXM 4.5.
func (c *Converter) From() version.Version { return version.AIXM45 }

// To implements convert.Converter: AIXM 5.1.1.
func (c *Converter) To() version.Version { return version.AIXM511 }

// ID implements convert.Converter.
func (c *Converter) ID() convert.ConverterID { return c.id }

// Convert implements convert.Converter.  It reads src twice: pass 1 indexes
// the natural keys (existence, duplicates) and the navaid collocations,
// pass 2 converts element by element.
func (c *Converter) Convert(ctx context.Context, src convert.Source, dst convert.Sink) (*convert.Report, error) {
	if c.cfg.Namespace.IsZero() {
		return nil, ErrNoNamespace
	}
	r := &run{cfg: c.cfg, ctx: ctx, rep: convert.NewReport(), dst: dst}
	if err := r.pass1(src); err != nil {
		return nil, err
	}
	if err := dst.Begin(convert.Header{Version: version.AIXM511, Root: "AIXMBasicMessage", GMLID: "msg-" + r.sha[:32], Attrs: r.h.Attrs}); err != nil {
		return nil, err
	}
	if err := r.pass2(src); err != nil {
		return nil, err
	}
	if err := dst.End(); err != nil {
		return nil, err
	}
	return r.rep, nil
}

// run is the state of one conversion.
type run struct {
	cfg Config
	ctx context.Context
	rep *convert.Report
	dst convert.Sink
	h   aixm45.Header
	sha string
	ts  timeSlice

	// keys counts the occurrences of every own natural key (all kinds).
	keys map[string]int32
	// seen counts occurrences while numbering keys in a pass.
	seen map[string]int32
	// colloc maps a Vor key to the Dme and Tcn collocated with it.
	colloc map[string][]member
	// grouped holds the own keys of Dme and Tcn that join a Vor's Navaid.
	grouped map[string]bool
}

type member struct {
	kind, key, mid string
	index          int
}

// Phase-1a kinds and their converters; phase-1b kinds.
var (
	mappedKinds = map[string]func(*elem) error{
		"Org": (*elem).org, "Uni": (*elem).uni, "Dpn": (*elem).dpn,
		"Vor": (*elem).vor, "Dme": (*elem).dme, "Ndb": (*elem).ndb, "Tcn": (*elem).tcn,
		"Obs": (*elem).obs, "Ahp": (*elem).ahp, "Rwy": (*elem).rwy, "Rdn": (*elem).rdn,
	}
	phase1b = map[string]bool{"Ase": true, "Abd": true, "Adg": true, "Avx": true, "Gbr": true, "Gbv": true, "Att": true, "Ain": true}
)

func open(src convert.Source, h io.Writer) (io.ReadCloser, io.Reader, error) {
	rc, err := src.Open()
	if err != nil {
		return nil, nil, err
	}
	if h == nil {
		return rc, rc, nil
	}
	return rc, io.TeeReader(rc, h), nil
}

// ownKey numbers the natural key of e's own …Uid in this pass: "" when the
// element has none.
func (r *run) ownKey(kind string, n *xmlnode.Node) (base, key string) {
	uid := n.Child(kind + "Uid")
	if uid == nil {
		return "", ""
	}
	base = aixm45.Key(uid)
	r.seen[base]++
	if c := r.seen[base]; c > 1 {
		return base, fmt.Sprintf("%s#%d", base, c)
	}
	return base, base
}

func (r *run) pass1(src convert.Source) error {
	hash := sha256.New()
	rc, in, err := open(src, hash)
	if err != nil {
		return err
	}
	defer rc.Close()
	r.keys, r.seen = map[string]int32{}, map[string]int32{}
	type pending struct {
		member
		vor string
	}
	var coll []pending
	h, err := aixm45.Stream(r.ctx, in, func(e aixm45.Element) error {
		kind := e.Kind()
		base, key := r.ownKey(kind, e.Node)
		if base == "" {
			return nil
		}
		r.keys[base]++
		if kind == "Dme" || kind == "Tcn" {
			if vor := e.Node.Child("VorUid"); vor != nil {
				mid, _ := e.Node.Child(kind + "Uid").Attr("mid")
				coll = append(coll, pending{member{kind, key, mid, e.Index}, aixm45.Key(vor)})
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, aixm45.ErrNotAIXM45) {
			return fmt.Errorf("%w: %v", convert.ErrVersionMismatch, err)
		}
		return err
	}
	if _, err := io.Copy(io.Discard, in); err != nil {
		return err
	}
	if h.Root == aixm45.RootUpdate {
		return ErrUpdate
	}
	if !validDateTime(h.Effective) {
		return fmt.Errorf("%w: %q", ErrNoEffective, h.Effective)
	}
	r.h, r.sha = h, hex.EncodeToString(hash.Sum(nil))
	r.ts = timeSlice{begin: h.Effective, sequence: 1}
	if r.cfg.Sequence != nil {
		r.ts.sequence = r.cfg.Sequence(h)
	}
	r.colloc, r.grouped = map[string][]member{}, map[string]bool{}
	for _, p := range coll {
		if r.keys[p.vor] > 0 && strings.HasPrefix(p.vor, "Vor(") {
			r.colloc[p.vor] = append(r.colloc[p.vor], p.member)
			r.grouped[p.key] = true
		}
	}
	// Duplicate keys of the kinds converted (D7).
	var dups []string
	for k, n := range r.keys {
		if n > 1 && mappedKinds[k[:strings.IndexByte(k, '(')]] != nil {
			dups = append(dups, k)
		}
	}
	sort.Strings(dups)
	for _, k := range dups {
		kind := k[:strings.IndexByte(k, '(')]
		r.rep.AddIssue(convert.Issue{Code: "duplicate_key", Severity: convert.Warning, Kind: kind, Key: k,
			Message: "natural key occurs more than once: later occurrences get #2, #3, … (identity depends on document order)", Count: int(r.keys[k] - 1)})
	}
	return nil
}

func validDateTime(s string) bool {
	if s == "" {
		return false
	}
	for _, layout := range []string{"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05.999999999Z07:00", "2006-01-02T15:04:05", "2006-01-02T15:04:05.999999999"} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

func (r *run) pass2(src convert.Source) error {
	rc, in, err := open(src, nil)
	if err != nil {
		return err
	}
	defer rc.Close()
	r.seen = map[string]int32{}
	r.header()
	_, err = aixm45.Stream(r.ctx, in, r.element)
	return err
}

// header accounts for the root element's attributes.
func (r *run) header() {
	k := r.rep.Kind(r.h.Root)
	k.In++
	k.Merged++ // folded into the message header and the time slices
	// Only the attributes: the root's content is the elements.
	k.Leaves += len(r.h.Node.Attrs)
	convert.WalkLeaves(r.h.Node, func(l convert.Leaf) {
		if l.Attr == "" {
			return
		}
		id := "HDR-OTHER"
		switch l.Attr {
		case "version":
			id = "HDR-VER"
		case "effective":
			id = "HDR-EFF"
		case "origin", "created":
			id = "HDR-ATTR"
		}
		ru := rule(id)
		var s *convert.Sample
		if ru.Outcome != convert.Mapped {
			s = &convert.Sample{Value: l.Value}
		}
		k.Record(l.Path, ru.Outcome, ru.ID, ru.Target, s)
	})
}

func (r *run) element(e aixm45.Element) error {
	kind := e.Kind()
	conv := mappedKinds[kind]
	el := &elem{r: r, kind: kind, n: e.Node, index: e.Index, ks: r.rep.Kind(kind), marks: map[leafID]mark{}}
	el.ks.In++
	switch {
	case conv == nil:
		el.fallback = "LATER"
		if phase1b[kind] {
			el.fallback = "PH1B"
		}
		el.ks.Dropped++
		if el.ks.In == 1 {
			r.rep.AddIssue(convert.Issue{Code: "kind_unmapped", Severity: convert.Info, Kind: kind, Message: rule(el.fallback).Note})
		}
		el.finish()
		return nil
	}
	el.fallback = "UNMAPPED"
	base, key := r.ownKey(kind, e.Node)
	if base == "" {
		el.fallback = "NO-UID"
		el.ks.Dropped++
		r.rep.AddIssue(convert.Issue{Code: "no_uid", Severity: convert.Error, Kind: kind, Message: "element without its " + kind + "Uid: dropped"})
		el.finish()
		return nil
	}
	el.base, el.key = base, key
	el.uuid = r.uuid(key)
	if err := conv(el); err != nil {
		return err
	}
	el.finish()
	return nil
}

func (r *run) uuid(key string) string {
	return convert.NewV5(r.cfg.Namespace, "aixm45:"+key).String()
}

// put emits a record of a feature.
func (r *run) put(f *feature, order []string, p convert.Provenance) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	return r.dst.Put(convert.Record{Kind: f.name, XML: f.bytes(r.ts, order), Provenance: p})
}

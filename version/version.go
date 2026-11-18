// Package version identifies AIXM schema versions and detects the version of
// a document from its root element.
//
// It is a leaf package (standard library only) shared by the decoder
// (package goaixm), the converter framework (package convert) and the
// readers of other AIXM versions.
//
//	d, err := version.DetectFile("data.xml")
//	if err == nil && d.Version == version.AIXM511 {
//		// native input for goaixm.Parse
//	}
package version

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Version is an AIXM schema version: 4.5 is {4, 5, 0}, 5.1.1 is {5, 1, 1}.
// The zero value means "unknown".
type Version struct{ Major, Minor, Patch int }

// Well-known AIXM versions.  AIXM511 is the version of package aixm and the
// native input of goaixm.Parse.
var (
	AIXM40  = Version{4, 0, 0}
	AIXM45  = Version{4, 5, 0}
	AIXM51  = Version{5, 1, 0}
	AIXM511 = Version{5, 1, 1}
	AIXM52  = Version{5, 2, 0}
)

// String formats v as "4.5" or "5.1.1": the patch level is omitted when it
// is zero.  The zero Version formats as "unknown".
func (v Version) String() string {
	if v.IsZero() {
		return "unknown"
	}
	if v.Patch == 0 {
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsZero reports whether v is the zero (unknown) version.
func (v Version) IsZero() bool { return v == Version{} }

// Compare returns -1, 0 or +1 when v is lower than, equal to or higher than o.
func (v Version) Compare(o Version) int {
	for _, d := range [3]int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		switch {
		case d < 0:
			return -1
		case d > 0:
			return 1
		}
	}
	return 0
}

// Less reports whether v is lower than o.
func (v Version) Less(o Version) bool { return v.Compare(o) < 0 }

// MarshalText encodes v as its String form, so Versions appear as "4.5" in
// JSON.
func (v Version) MarshalText() ([]byte, error) { return []byte(v.String()), nil }

// UnmarshalText parses the String form ("unknown" gives the zero Version).
func (v *Version) UnmarshalText(b []byte) error {
	if string(b) == "unknown" || len(b) == 0 {
		*v = Version{}
		return nil
	}
	p, err := Parse(string(b))
	if err != nil {
		return err
	}
	*v = p
	return nil
}

// Parse parses "4.5", "5.1" or "5.1.1" (two or three non-negative decimal
// components).
func Parse(s string) (Version, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("version: invalid version %q", s)
	}
	var n [3]int
	for i, p := range parts {
		if p == "" || len(p) > 4 || strings.TrimLeft(p, "0123456789") != "" {
			return Version{}, fmt.Errorf("version: invalid version %q", s)
		}
		n[i], _ = strconv.Atoi(p)
	}
	return Version{n[0], n[1], n[2]}, nil
}

// ErrUnknownVersion is returned by Detect for documents whose root element
// does not identify a known AIXM version.
var ErrUnknownVersion = errors.New("version: not an AIXM document of a known version")

// Detected is what the root element of a document tells.
type Detected struct {
	// Version is the detected AIXM version; zero when unknown.
	Version Version `json:"version"`
	// Root is the local name of the root element: AIXM-Snapshot,
	// AIXMBasicMessage, ADRMessage, …
	Root string `json:"root"`
	// Namespace is the namespace of the root element, "" for AIXM 4.x.
	Namespace string `json:"namespace,omitempty"`
	// Attrs holds the root element's attributes by local name, namespace
	// declarations excluded (4.5: version, origin, created, effective).
	Attrs map[string]string `json:"attrs,omitempty"`
}

// Detect reads r up to the first start element and detects the document's
// AIXM version.  The reader is read ahead in blocks, so callers that need
// the document afterwards must re-open it or buffer what was read.
//
// The rules, in order:
//
//  1. Root AIXM-Snapshot or AIXM-Update without namespace: AIXM 4.x, the
//     version attribute gives the minor version ("4.5").  A missing or
//     malformed attribute gives an error wrapping ErrUnknownVersion, with
//     Root still set.
//  2. Root in an AIXM message namespace (http://www.aixm.aero/schema/5.1.1/message)
//     or an AIXM namespace (http://www.aixm.aero/schema/5.1.1): that version.
//  3. Otherwise a declared AIXM namespace on the root, the one bound to the
//     prefix "aixm" first, then the default namespace, then the others in
//     document order (this decides ADRMessage, whose own namespace is not
//     an AIXM one).
//  4. Otherwise ErrUnknownVersion (Root and Namespace still set).
func Detect(r io.Reader) (Detected, error) {
	d := xml.NewDecoder(bufio.NewReader(r))
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return Detected{}, fmt.Errorf("%w: no root element", ErrUnknownVersion)
			}
			return Detected{}, fmt.Errorf("%w: %v", ErrUnknownVersion, err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return FromStart(se)
		}
	}
}

// DetectFile detects the AIXM version of the file at path.
func DetectFile(path string) (Detected, error) {
	f, err := os.Open(path)
	if err != nil {
		return Detected{}, err
	}
	defer f.Close()
	return Detect(f)
}

// FromStart detects the version from a root start element read with
// xml.Decoder.Token.  Token keeps the namespace declarations among the
// attributes, so the rules of Detect apply unchanged.  Decoders that already
// read the root (such as goaixm's) use it instead of reading twice.
func FromStart(se xml.StartElement) (Detected, error) {
	decls := map[string]string{}
	var order []string
	attrs := map[string]string{}
	for _, a := range se.Attr {
		switch {
		case a.Name.Space == "xmlns":
			decls[a.Name.Local] = a.Value
			order = append(order, a.Name.Local)
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			decls[""] = a.Value
			order = append(order, "")
		default:
			if _, dup := attrs[a.Name.Local]; !dup {
				attrs[a.Name.Local] = a.Value
			}
		}
	}
	return classify(se.Name.Local, se.Name.Space, attrs, decls, order)
}

// IsAIXM4Root reports whether local is the root element name of an AIXM 4.x
// document (AIXM-Snapshot or AIXM-Update).
func IsAIXM4Root(local string) bool {
	return local == "AIXM-Snapshot" || local == "AIXM-Update"
}

func classify(root, ns string, attrs, decls map[string]string, order []string) (Detected, error) {
	det := Detected{Root: root, Namespace: ns}
	if len(attrs) > 0 {
		det.Attrs = attrs
	}
	if ns == "" && IsAIXM4Root(root) {
		v, err := Parse(attrs["version"])
		if err != nil || v.Major != 4 {
			return det, fmt.Errorf("%w: %s with version attribute %q", ErrUnknownVersion, root, attrs["version"])
		}
		det.Version = v
		return det, nil
	}
	if v, ok := Namespace(ns); ok {
		det.Version = v
		return det, nil
	}
	candidates := append([]string{"aixm", ""}, order...)
	for _, p := range candidates {
		if uri, ok := decls[p]; ok {
			if v, ok := Namespace(uri); ok {
				det.Version = v
				return det, nil
			}
		}
	}
	return det, ErrUnknownVersion
}

const nsPrefix = "http://www.aixm.aero/schema/"

// Namespace returns the AIXM 5.x version named by an AIXM namespace URI:
// http://www.aixm.aero/schema/5.1.1 and its /message sub-namespace give
// 5.1.1.  Other sub-namespaces (extensions) are not AIXM namespaces.
func Namespace(uri string) (Version, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(uri), nsPrefix)
	if !ok {
		return Version{}, false
	}
	rest = strings.TrimSuffix(rest, "/")
	rest = strings.TrimSuffix(rest, "/message")
	if strings.Contains(rest, "/") {
		return Version{}, false
	}
	v, err := Parse(rest)
	if err != nil || v.Major < 5 {
		return Version{}, false
	}
	return v, true
}

// NamespaceURI returns the AIXM namespace of a 5.x version
// ("http://www.aixm.aero/schema/5.1.1").
func NamespaceURI(v Version) string { return nsPrefix + v.String() }

// MessageNamespaceURI returns the AIXM message namespace of a 5.x version
// ("http://www.aixm.aero/schema/5.1.1/message").
func MessageNamespaceURI(v Version) string { return NamespaceURI(v) + "/message" }

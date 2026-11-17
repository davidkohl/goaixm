package aixm45

import (
	"sort"
	"strings"

	"github.com/davidkohl/goaixm/xmlnode"
)

// KindOf returns the entity a …Uid element names: the part of its local
// name before "Uid" ("AhpUid" and "AhpUidAssoc" give "Ahp", "RwyUid" gives
// "Rwy").  It returns "" for names without "Uid".
func KindOf(uidName string) string {
	i := strings.Index(uidName, "Uid")
	if i <= 0 {
		return ""
	}
	return uidName[:i]
}

// IsUid reports whether n is a …Uid element.
func IsUid(n *xmlnode.Node) bool { return n != nil && KindOf(n.Local()) != "" }

// Key returns the canonical natural key of a …Uid element: the entity kind
// and the key children, e.g.
//
//	Rdn(RwyUid(AhpUid(codeId=XA01);txtDesig=09/27);txtDesig=27)
//
// Rules:
//
//   - the kind is KindOf the element name, so a reference (AhpUidAssoc) and
//     the referenced element's own AhpUid give the same key;
//   - children are sorted by local name (then by their rendering), so the
//     key does not depend on element order;
//   - a child with children is rendered as name(…) with the same rules;
//   - text is trimmed and inner white space collapsed to one space; case is
//     kept;
//   - geoLat… and geoLong… values that parse as 4.5 coordinates are
//     replaced by Coord.Canonical, so equal positions in different layouts
//     give one key;
//   - attributes (mid) are not part of the key;
//   - the characters \ ( ) ; = in values are escaped with a backslash.
//
// Key of a nil node is "".
func Key(uid *xmlnode.Node) string {
	if uid == nil {
		return ""
	}
	kind := KindOf(uid.Local())
	if kind == "" {
		kind = uid.Local()
	}
	return kind + "(" + keyParts(uid) + ")"
}

func keyParts(n *xmlnode.Node) string {
	type part struct{ name, s string }
	parts := make([]part, 0, len(n.Children))
	for _, c := range n.Children {
		name := c.Local()
		if len(c.Children) > 0 {
			parts = append(parts, part{name, escape(name) + "(" + keyParts(c) + ")"})
			continue
		}
		parts = append(parts, part{name, escape(name) + "=" + escape(KeyValue(name, c.Text))})
	}
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].name != parts[j].name {
			return parts[i].name < parts[j].name
		}
		return parts[i].s < parts[j].s
	})
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(p.s)
	}
	return b.String()
}

// KeyValue returns the canonical form of a key child's text (see Key).
func KeyValue(name, text string) string {
	v := strings.Join(strings.Fields(text), " ")
	switch {
	case strings.HasPrefix(name, "geoLat"):
		if c, err := ParseLat(v); err == nil {
			return c.Canonical()
		}
	case strings.HasPrefix(name, "geoLong"):
		if c, err := ParseLong(v); err == nil {
			return c.Canonical()
		}
	}
	return v
}

func escape(s string) string {
	if !strings.ContainsAny(s, `\();=`) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '(', ')', ';', '=':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

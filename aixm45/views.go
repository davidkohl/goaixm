package aixm45

import "github.com/davidkohl/goaixm/xmlnode"

// Value is one value of a view: the element's trimmed text and the element
// itself (Node nil when the element is absent).  Views take the first
// element of a name; repeated elements stay in the tree.
type Value struct {
	Text string
	Node *xmlnode.Node
}

// OK reports whether the element is present.
func (v Value) OK() bool { return v.Node != nil }

// Ref is a …Uid element: the identity of an element, or a reference to
// another one.  Kind and Key follow KindOf and Key; MID is the optional
// producer id attribute.
type Ref struct {
	Node      *xmlnode.Node
	Kind, Key string
	MID       string
}

// OK reports whether the element is present.
func (r Ref) OK() bool { return r.Node != nil }

// Position is a horizontal position: geoLat, geoLong and codeDatum.
type Position struct {
	Lat, Long, Datum Value
}

// Elevation is the vertical part of a position: valElev, valElevAccuracy,
// valGeoidUndulation and their unit uomDistVer.
type Elevation struct {
	Elev, Accuracy, GeoidUndulation, Uom Value
}

func val(n *xmlnode.Node, path string) Value {
	if n == nil {
		return Value{}
	}
	c := n.Find(path)
	if c == nil {
		return Value{}
	}
	return Value{Text: c.Text, Node: c}
}

func ref(n *xmlnode.Node, path string) Ref {
	if n == nil {
		return Ref{}
	}
	c := n.Find(path)
	if c == nil {
		return Ref{}
	}
	mid, _ := c.Attr("mid")
	return Ref{Node: c, Kind: KindOf(c.Local()), Key: Key(c), MID: mid}
}

func elevation(n *xmlnode.Node) Elevation {
	return Elevation{Elev: val(n, "valElev"), Accuracy: val(n, "valElevAccuracy"), GeoidUndulation: val(n, "valGeoidUndulation"), Uom: val(n, "uomDistVer")}
}

// Common holds the values most entities share.
type Common struct {
	// Uid is the element's own …Uid.
	Uid Ref
	// Remark is txtRmk, Crc is valCrc.
	Remark, Crc Value
}

func common(n *xmlnode.Node, uid string) Common {
	return Common{Uid: ref(n, uid), Remark: val(n, "txtRmk"), Crc: val(n, "valCrc")}
}

// Org is an organisation or authority.
type Org struct {
	Common
	Name             Value // OrgUid/txtName
	CodeID, CodeType Value
}

// NewOrg returns the view of an Org element.
func NewOrg(n *xmlnode.Node) Org {
	return Org{Common: common(n, "OrgUid"), Name: val(n, "OrgUid/txtName"), CodeID: val(n, "codeId"), CodeType: val(n, "codeType")}
}

// Uni is an ATS or other unit.
type Uni struct {
	Common
	Name                Value // UniUid/txtName
	Org, Ahp            Ref
	CodeType, CodeClass Value
	Position            Position
}

// NewUni returns the view of a Uni element.
func NewUni(n *xmlnode.Node) Uni {
	return Uni{
		Common: common(n, "UniUid"), Name: val(n, "UniUid/txtName"),
		Org: ref(n, "OrgUid"), Ahp: ref(n, "AhpUid"),
		CodeType: val(n, "codeType"), CodeClass: val(n, "codeClass"),
		Position: Position{Lat: val(n, "geoLat"), Long: val(n, "geoLong"), Datum: val(n, "codeDatum")},
	}
}

// Dpn is a designated point.
type Dpn struct {
	Common
	CodeID         Value // DpnUid/codeId
	Position       Position
	CodeType, Name Value
	// AhpAssoc is AhpUidAssoc, the associated aerodrome; Ahp is an AhpUid
	// child, which some producers write instead.
	AhpAssoc, Ahp Ref
}

// NewDpn returns the view of a Dpn element.
func NewDpn(n *xmlnode.Node) Dpn {
	return Dpn{
		Common: common(n, "DpnUid"), CodeID: val(n, "DpnUid/codeId"),
		Position: Position{Lat: val(n, "DpnUid/geoLat"), Long: val(n, "DpnUid/geoLong"), Datum: val(n, "codeDatum")},
		CodeType: val(n, "codeType"), Name: val(n, "txtName"),
		AhpAssoc: ref(n, "AhpUidAssoc"), Ahp: ref(n, "AhpUid"),
	}
}

// Equipment holds what the navaid equipment entities (Vor, Dme, Ndb, Tcn)
// share.
type Equipment struct {
	Common
	CodeID    Value // …Uid/codeId
	Position  Position
	Elevation Elevation
	Org       Ref
	Name      Value
	// MagVar is valMagVar, MagVarDate dateMagVar.
	MagVar, MagVarDate Value
}

func equipment(n *xmlnode.Node, uid string) Equipment {
	return Equipment{
		Common: common(n, uid), CodeID: val(n, uid+"/codeId"),
		Position:  Position{Lat: val(n, uid+"/geoLat"), Long: val(n, uid+"/geoLong"), Datum: val(n, "codeDatum")},
		Elevation: elevation(n), Org: ref(n, "OrgUid"), Name: val(n, "txtName"),
		MagVar: val(n, "valMagVar"), MagVarDate: val(n, "dateMagVar"),
	}
}

// Vor is a VOR.
type Vor struct {
	Equipment
	CodeType, Freq, FreqUom, CodeTypeNorth, Declination Value
}

// NewVor returns the view of a Vor element.
func NewVor(n *xmlnode.Node) Vor {
	return Vor{Equipment: equipment(n, "VorUid"), CodeType: val(n, "codeType"), Freq: val(n, "valFreq"), FreqUom: val(n, "uomFreq"),
		CodeTypeNorth: val(n, "codeTypeNorth"), Declination: val(n, "valDeclination")}
}

// Dme is a DME; Vor is the collocated VOR (VorUid), if any.
type Dme struct {
	Equipment
	Vor                                                               Ref
	CodeType, Channel, GhostFreq, GhostFreqUom, Displace, DisplaceUom Value
}

// NewDme returns the view of a Dme element.
func NewDme(n *xmlnode.Node) Dme {
	return Dme{Equipment: equipment(n, "DmeUid"), Vor: ref(n, "VorUid"), CodeType: val(n, "codeType"), Channel: val(n, "codeChannel"),
		GhostFreq: val(n, "valGhostFreq"), GhostFreqUom: val(n, "uomGhostFreq"), Displace: val(n, "valDisplace"), DisplaceUom: val(n, "uomDisplace")}
}

// Ndb is an NDB.
type Ndb struct {
	Equipment
	Freq, FreqUom, CodeClass, CodeEm Value
}

// NewNdb returns the view of an Ndb element.
func NewNdb(n *xmlnode.Node) Ndb {
	return Ndb{Equipment: equipment(n, "NdbUid"), Freq: val(n, "valFreq"), FreqUom: val(n, "uomFreq"), CodeClass: val(n, "codeClass"), CodeEm: val(n, "codeEm")}
}

// Tcn is a TACAN; Vor is the collocated VOR (VorUid), if any.
type Tcn struct {
	Equipment
	Vor                  Ref
	Channel, Declination Value
}

// NewTcn returns the view of a Tcn element.
func NewTcn(n *xmlnode.Node) Tcn {
	return Tcn{Equipment: equipment(n, "TcnUid"), Vor: ref(n, "VorUid"), Channel: val(n, "codeChannel"), Declination: val(n, "valDeclination")}
}

// Obs is an obstacle.  Its natural key is its position (ObsUid geoLat,
// geoLong).
type Obs struct {
	Common
	Position                            Position
	Elevation                           Elevation
	Name, DescrType, CodeGroup, CodeLgt Value
	DescrLgt, DescrMarking, Height      Value // txtDescrLgt, txtDescrMarking, valHgt
	GeoAccuracy, GeoAccuracyUom         Value // valGeoAccuracy, uomGeoAccuracy
}

// NewObs returns the view of an Obs element.
func NewObs(n *xmlnode.Node) Obs {
	return Obs{
		Common:    common(n, "ObsUid"),
		Position:  Position{Lat: val(n, "ObsUid/geoLat"), Long: val(n, "ObsUid/geoLong"), Datum: val(n, "codeDatum")},
		Elevation: elevation(n),
		Name:      val(n, "txtName"), DescrType: val(n, "txtDescrType"), CodeGroup: val(n, "codeGroup"), CodeLgt: val(n, "codeLgt"),
		DescrLgt: val(n, "txtDescrLgt"), DescrMarking: val(n, "txtDescrMarking"), Height: val(n, "valHgt"),
		GeoAccuracy: val(n, "valGeoAccuracy"), GeoAccuracyUom: val(n, "uomGeoAccuracy"),
	}
}

// Ahp is an aerodrome or heliport.
type Ahp struct {
	Common
	CodeID                                        Value // AhpUid/codeId
	Org                                           Ref
	Name, CodeIcao, CodeIata, CodeType            Value
	Position                                      Position
	Elevation                                     Elevation
	MagVar, MagVarDate, MagVarChange              Value // valMagVar, dateMagVar, valMagVarChg
	RefTemp, RefTempUom                           Value // valRefT, uomRefT
	TransitionAlt, TransitionAltUom               Value
	NameCityServed, NameAdmin                     Value // txtNameCitySer, txtNameAdmin
	DescrRefPoint, DescrSite, DescrAltimeterCheck Value // txtDescrRefPt, txtDescrSite, txtDescrAcl
}

// NewAhp returns the view of an Ahp element.
func NewAhp(n *xmlnode.Node) Ahp {
	return Ahp{
		Common: common(n, "AhpUid"), CodeID: val(n, "AhpUid/codeId"), Org: ref(n, "OrgUid"),
		Name: val(n, "txtName"), CodeIcao: val(n, "codeIcao"), CodeIata: val(n, "codeIata"), CodeType: val(n, "codeType"),
		Position:  Position{Lat: val(n, "geoLat"), Long: val(n, "geoLong"), Datum: val(n, "codeDatum")},
		Elevation: elevation(n),
		MagVar:    val(n, "valMagVar"), MagVarDate: val(n, "dateMagVar"), MagVarChange: val(n, "valMagVarChg"),
		RefTemp: val(n, "valRefT"), RefTempUom: val(n, "uomRefT"),
		TransitionAlt: val(n, "valTransitionAlt"), TransitionAltUom: val(n, "uomTransitionAlt"),
		NameCityServed: val(n, "txtNameCitySer"), NameAdmin: val(n, "txtNameAdmin"),
		DescrRefPoint: val(n, "txtDescrRefPt"), DescrSite: val(n, "txtDescrSite"), DescrAltimeterCheck: val(n, "txtDescrAcl"),
	}
}

// Rwy is a runway (or FATO strip) of an aerodrome.
type Rwy struct {
	Common
	Desig                                   Value // RwyUid/txtDesig
	Ahp                                     Ref   // RwyUid/AhpUid
	Len, Wid, DimUom                        Value // valLen, valWid, uomDimRwy
	LenStrip, WidStrip, DimStripUom         Value // valLenStrip, valWidStrip, uomDimStrip
	Composition                             Value // codeComposition
	PcnClass, PcnPavementType, PcnSubgrade  Value // valPcnClass, codePcnPavementType, codePcnPavementSubgrade
	PcnTyrePressure, PcnEvalMethod, PcnNote Value // codePcnMaxTirePressure, codePcnEvalMethod, txtPcnNote
	Profile, Marking                        Value // txtProfile, txtMarking
}

// NewRwy returns the view of a Rwy element.
func NewRwy(n *xmlnode.Node) Rwy {
	return Rwy{
		Common: common(n, "RwyUid"), Desig: val(n, "RwyUid/txtDesig"), Ahp: ref(n, "RwyUid/AhpUid"),
		Len: val(n, "valLen"), Wid: val(n, "valWid"), DimUom: val(n, "uomDimRwy"),
		LenStrip: val(n, "valLenStrip"), WidStrip: val(n, "valWidStrip"), DimStripUom: val(n, "uomDimStrip"),
		Composition: val(n, "codeComposition"),
		PcnClass:    val(n, "valPcnClass"), PcnPavementType: val(n, "codePcnPavementType"), PcnSubgrade: val(n, "codePcnPavementSubgrade"),
		PcnTyrePressure: val(n, "codePcnMaxTirePressure"), PcnEvalMethod: val(n, "codePcnEvalMethod"), PcnNote: val(n, "txtPcnNote"),
		Profile: val(n, "txtProfile"), Marking: val(n, "txtMarking"),
	}
}

// Rdn is a runway direction.
type Rdn struct {
	Common
	Desig               Value // RdnUid/txtDesig
	Rwy                 Ref   // RdnUid/RwyUid
	TrueBrg, MagBrg     Value // valTrueBrg, valMagBrg
	ElevTdz, ElevTdzUom Value // valElevTdz, uomElevTdz
	Position            Position
}

// NewRdn returns the view of a Rdn element.
func NewRdn(n *xmlnode.Node) Rdn {
	return Rdn{
		Common: common(n, "RdnUid"), Desig: val(n, "RdnUid/txtDesig"), Rwy: ref(n, "RdnUid/RwyUid"),
		TrueBrg: val(n, "valTrueBrg"), MagBrg: val(n, "valMagBrg"),
		ElevTdz: val(n, "valElevTdz"), ElevTdzUom: val(n, "uomElevTdz"),
		Position: Position{Lat: val(n, "geoLat"), Long: val(n, "geoLong"), Datum: val(n, "codeDatum")},
	}
}

// Limit is one vertical limit of an airspace: codeDistVer… (reference),
// valDistVer… (value) and uomDistVer… (unit), e.g. Upper.
type Limit struct {
	Code, Val, Uom Value
}

// OK reports whether any part of the limit is present.
func (l Limit) OK() bool { return l.Code.OK() || l.Val.OK() || l.Uom.OK() }

func limit(n *xmlnode.Node, suffix string) Limit {
	return Limit{Code: val(n, "codeDistVer"+suffix), Val: val(n, "valDistVer"+suffix), Uom: val(n, "uomDistVer"+suffix)}
}

// Ase is an airspace (its properties; the horizontal shape is an Abd or
// an Adg).
type Ase struct {
	Common
	CodeType, CodeID           Value // AseUid/codeType, AseUid/codeId
	LocalType, Name, CodeClass Value // txtLocalType, txtName, codeClass
	CodeActivity, CodeMil      Value // codeActivity, codeMil
	CodeLocInd                 Value // codeLocInd
	Upper, Lower, Max, Min     Limit // …Upper, …Lower, …Max, …Mnm
	// Att holds the working-hours elements (Att with codeWorkHr,
	// txtRmkWorkHr and Timsh children).
	Att []*xmlnode.Node
}

// NewAse returns the view of an Ase element.
func NewAse(n *xmlnode.Node) Ase {
	return Ase{
		Common: common(n, "AseUid"), CodeType: val(n, "AseUid/codeType"), CodeID: val(n, "AseUid/codeId"),
		LocalType: val(n, "txtLocalType"), Name: val(n, "txtName"), CodeClass: val(n, "codeClass"),
		CodeActivity: val(n, "codeActivity"), CodeMil: val(n, "codeMil"), CodeLocInd: val(n, "codeLocInd"),
		Upper: limit(n, "Upper"), Lower: limit(n, "Lower"), Max: limit(n, "Max"), Min: limit(n, "Mnm"),
		Att: n.ChildrenNamed("Att"),
	}
}

// Avx is one vertex of an airspace boundary (Abd) and the curve from it
// to the next vertex: codeType GRC (great circle), RHL (rhumb line), CWA
// and CCA (clockwise and counter-clockwise arc about geoLatArc/geoLongArc
// with valRadiusArc), FNT (along the border GbrUid).
type Avx struct {
	Node              *xmlnode.Node
	CodeType          Value
	Position          Position
	ArcLat, ArcLong   Value // geoLatArc, geoLongArc
	Radius, RadiusUom Value // valRadiusArc, uomRadiusArc
	Gbr               Ref   // GbrUid
	Remark, Crc       Value // txtRmk, valCrc
}

// NewAvx returns the view of an Avx (or Gbv) element.
func NewAvx(n *xmlnode.Node) Avx {
	return Avx{
		Node: n, CodeType: val(n, "codeType"),
		Position: Position{Lat: val(n, "geoLat"), Long: val(n, "geoLong"), Datum: val(n, "codeDatum")},
		ArcLat:   val(n, "geoLatArc"), ArcLong: val(n, "geoLongArc"),
		Radius: val(n, "valRadiusArc"), RadiusUom: val(n, "uomRadiusArc"),
		Gbr: ref(n, "GbrUid"), Remark: val(n, "txtRmk"), Crc: val(n, "valCrc"),
	}
}

// Circle is a circular airspace boundary (Abd/Circle).
type Circle struct {
	Node              *xmlnode.Node
	Centre            Position // geoLatCen, geoLongCen, codeDatum
	Radius, RadiusUom Value    // valRadius, uomRadius
	Remark, Crc       Value
}

// Abd is the horizontal boundary of an airspace: a ring of vertices or a
// circle.
type Abd struct {
	Common
	// Ase is the airspace (AbdUid/AseUid).
	Ase    Ref
	Vertex []Avx
	Circle *Circle
}

// NewAbd returns the view of an Abd element.
func NewAbd(n *xmlnode.Node) Abd {
	a := Abd{Common: common(n, "AbdUid"), Ase: ref(n, "AbdUid/AseUid")}
	for _, v := range n.ChildrenNamed("Avx") {
		a.Vertex = append(a.Vertex, NewAvx(v))
	}
	if c := n.Child("Circle"); c != nil {
		a.Circle = &Circle{Node: c,
			Centre: Position{Lat: val(c, "geoLatCen"), Long: val(c, "geoLongCen"), Datum: val(c, "codeDatum")},
			Radius: val(c, "valRadius"), RadiusUom: val(c, "uomRadius"), Remark: val(c, "txtRmk"), Crc: val(c, "valCrc")}
	}
	return a
}

// Gbr is a geographical border; its vertices (Gbv) have the Avx layout
// (codeType GRC, RHL, END for the last).
type Gbr struct {
	Common
	Name, CodeType Value // GbrUid/txtName, codeType
	Vertex         []Avx
}

// NewGbr returns the view of a Gbr element.
func NewGbr(n *xmlnode.Node) Gbr {
	g := Gbr{Common: common(n, "GbrUid"), Name: val(n, "GbrUid/txtName"), CodeType: val(n, "codeType")}
	for _, v := range n.ChildrenNamed("Gbv") {
		g.Vertex = append(g.Vertex, NewAvx(v))
	}
	return g
}

// AdgPart is one operand of an airspace derivation: the operator (codeOpr)
// preceding a component airspace.
type AdgPart struct {
	Opr Value // codeOpr; Node nil for the base
	Ase Ref
}

// Adg is the derived geometry of an airspace (AdgUid/AseUid): either the
// same extent as another airspace (AseUidSameExtent) or a base airspace
// (AseUidBase) combined with components (codeOpr, AseUidComponent pairs,
// in document order).
type Adg struct {
	Common
	Ase        Ref
	SameExtent Ref
	Base       Ref
	Parts      []AdgPart
	// Stray holds codeOpr elements not followed by a component.
	Stray []Value
}

// NewAdg returns the view of an Adg element.
func NewAdg(n *xmlnode.Node) Adg {
	a := Adg{Common: common(n, "AdgUid"), Ase: ref(n, "AdgUid/AseUid"), SameExtent: ref(n, "AseUidSameExtent"), Base: ref(n, "AseUidBase")}
	var opr *Value
	for _, c := range n.Children {
		switch c.Local() {
		case "codeOpr":
			if opr != nil {
				a.Stray = append(a.Stray, *opr)
			}
			opr = &Value{Text: c.Text, Node: c}
		case "AseUidComponent":
			p := AdgPart{Ase: refOf(c)}
			if opr != nil {
				p.Opr, opr = *opr, nil
			}
			a.Parts = append(a.Parts, p)
		}
	}
	if opr != nil {
		a.Stray = append(a.Stray, *opr)
	}
	return a
}

func refOf(c *xmlnode.Node) Ref {
	mid, _ := c.Attr("mid")
	return Ref{Node: c, Kind: KindOf(c.Local()), Key: Key(c), MID: mid}
}

// Timsh is one timetable row of working hours (Att/Timsh).
type Timsh struct {
	Node                                    *xmlnode.Node
	TimeRef, DateWef, DateTil, Day, DayTil  Value // codeTimeRef, dateValidWef, dateValidTil, codeDay, codeDayTil
	TimeWef, EventWef, RelEventWef, CombWef Value // timeWef, codeEventWef, timeRelEventWef, codeCombWef
	TimeTil, EventTil, RelEventTil, CombTil Value // timeTil, codeEventTil, timeRelEventTil, codeCombTil
}

// NewTimsh returns the view of a Timsh element.
func NewTimsh(n *xmlnode.Node) Timsh {
	return Timsh{Node: n, TimeRef: val(n, "codeTimeRef"), DateWef: val(n, "dateValidWef"), DateTil: val(n, "dateValidTil"),
		Day: val(n, "codeDay"), DayTil: val(n, "codeDayTil"),
		TimeWef: val(n, "timeWef"), EventWef: val(n, "codeEventWef"), RelEventWef: val(n, "timeRelEventWef"), CombWef: val(n, "codeCombWef"),
		TimeTil: val(n, "timeTil"), EventTil: val(n, "codeEventTil"), RelEventTil: val(n, "timeRelEventTil"), CombTil: val(n, "codeCombTil")}
}

// Att is a working-hours element: codeWorkHr (H24, HJ, HN, HX, HO, NOTAM,
// TIMSH, OTHER), its remark and, for TIMSH, the timetable.
type Att struct {
	Node              *xmlnode.Node
	WorkHr, RmkWorkHr Value
	Timsh             []Timsh
}

// NewAtt returns the view of an Att (or another timetable) element.
func NewAtt(n *xmlnode.Node) Att {
	a := Att{Node: n, WorkHr: val(n, "codeWorkHr"), RmkWorkHr: val(n, "txtRmkWorkHr")}
	for _, t := range n.ChildrenNamed("Timsh") {
		a.Timsh = append(a.Timsh, NewTimsh(t))
	}
	return a
}

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

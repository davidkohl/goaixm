package aixm45to51

import (
	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
)

// The per-kind conversions.  Each names its rules by id (mapping.tsv); a
// leaf none of them marks is unmapped (UNMAPPED).

func (e *elem) org() error {
	v := aixm45.NewOrg(e.n)
	f := newFeature("OrganisationAuthority", e.uuid)
	p := f.props
	e.str(p, "name", v.Name, "ORG-NAME", ttName)
	e.str(p, "designator", v.CodeID, "ORG-DESIG", ttOrgDesig)
	e.code(p, "type", v.CodeType, "ORG-TYPE", "org_type")
	e.common(p, v.Common)
	return e.emit(f, orderOrg)
}

func (e *elem) uni() error {
	v := aixm45.NewUni(e.n)
	f := newFeature("Unit", e.uuid)
	p := f.props
	e.str(p, "name", v.Name, "UNI-NAME", ttName)
	e.code(p, "type", v.CodeType, "UNI-TYPE", "unit_type")
	e.mark(v.CodeClass, "UNI-CLASS")
	if pp := e.position(f, v.Position, "UNI-POS"); pp != nil {
		p.add("position", wrap("position", point(pp, true)))
	}
	if u, ok := e.ref(v.Ahp, "UNI-AHP"); ok {
		p.add("airportLocation", href("airportLocation", u))
	}
	if u, ok := e.ref(v.Org, "UNI-ORG"); ok {
		p.add("ownerOrganisation", href("ownerOrganisation", u))
	}
	e.common(p, v.Common)
	return e.emit(f, orderUnit)
}

func (e *elem) dpn() error {
	v := aixm45.NewDpn(e.n)
	f := newFeature("DesignatedPoint", e.uuid)
	p := f.props
	e.str(p, "designator", v.CodeID, "DPN-DESIG", ttDpnDesig)
	e.code(p, "type", v.CodeType, "DPN-TYPE", "dpn_type")
	e.str(p, "name", v.Name, "DPN-NAME", ttName)
	if pp := e.position(f, v.Position, "DPN-POS"); pp != nil {
		e.measure(pp, "horizontalAccuracy", v.GeoAccuracy, v.GeoAccuracyUom, "GEO-HACC", ntDistance, "uom_dist")
		p.add("location", wrap("location", point(pp, false)))
	}
	// AhpUidAssoc, else AhpUid; airportHeliport takes one value.
	if u, ok := e.ref(v.AhpAssoc, "DPN-AHP"); ok {
		p.add("airportHeliport", href("airportHeliport", u))
		e.markTree(v.Ahp.Node, "DPN-AHP2", convert.Unmapped)
	} else if u, ok := e.ref(v.Ahp, "DPN-AHP2"); ok {
		p.add("airportHeliport", href("airportHeliport", u))
	}
	e.common(p, v.Common)
	return e.emit(f, orderDpn)
}

func (e *elem) vor() error {
	v := aixm45.NewVor(e.n)
	f := newFeature("VOR", e.uuid)
	p := f.props
	info := e.equipment(f, v.Equipment)
	e.code(p, "type", v.CodeType, "VOR-TYPE", "vor_type")
	e.measure(p, "frequency", v.Freq, v.FreqUom, "VOR-FREQ", ntFrequency, "uom_freq")
	e.code(p, "zeroBearingDirection", v.CodeTypeNorth, "VOR-NORTH", "north_ref")
	e.num(p, "declination", v.Declination, "VOR-DECL", ntMagVar)
	e.timesheets("Vtt")
	if err := e.emit(f, orderVOR); err != nil {
		return err
	}
	var others []member
	if e.key == e.base {
		others = e.r.colloc[e.key]
	}
	typ := "VOR"
	for _, m := range others {
		switch {
		case m.kind == "Tcn":
			typ = "VORTAC"
		case m.kind == "Dme" && typ == "VOR":
			typ = "VOR_DME"
		}
	}
	return e.navaid(typ, info, others)
}

// collocated handles the VorUid of a Dme or Tcn: true when the equipment
// joins the VOR's Navaid.
func (e *elem) collocated(vor aixm45.Ref, ruleID string) bool {
	if !vor.OK() {
		return false
	}
	if e.r.grouped[e.key] {
		e.markTree(vor.Node, ruleID, convert.Derived)
		return true
	}
	e.markTree(vor.Node, ruleID, convert.Unmapped)
	e.issue("dangling_ref", convert.Warning, vor.Key, "collocated "+vor.Kind+" is not in the document: equipment gets its own Navaid")
	return false
}

func (e *elem) dme() error {
	v := aixm45.NewDme(e.n)
	f := newFeature("DME", e.uuid)
	p := f.props
	info := e.equipment(f, v.Equipment)
	e.code(p, "type", v.CodeType, "DME-TYPE", "dme_type")
	e.str(p, "channel", v.Channel, "DME-CHAN", ttChannel)
	e.measure(p, "ghostFrequency", v.GhostFreq, v.GhostFreqUom, "DME-GHOST", ntFrequency, "uom_freq")
	e.measure(p, "displace", v.Displace, v.DisplaceUom, "DME-DISPL", ntDistance, "uom_dist")
	e.timesheets("Dtt")
	grouped := e.collocated(v.Vor, "DME-VOR")
	if err := e.emit(f, orderDME); err != nil {
		return err
	}
	if grouped {
		return nil
	}
	return e.navaid("DME", info, nil)
}

func (e *elem) tcn() error {
	v := aixm45.NewTcn(e.n)
	f := newFeature("TACAN", e.uuid)
	p := f.props
	info := e.equipment(f, v.Equipment)
	e.str(p, "channel", v.Channel, "TCN-CHAN", ttTACANChan)
	e.num(p, "declination", v.Declination, "TCN-DECL", ntMagVar)
	e.timesheets("Ttt")
	grouped := e.collocated(v.Vor, "TCN-VOR")
	if err := e.emit(f, orderTACAN); err != nil {
		return err
	}
	if grouped {
		return nil
	}
	return e.navaid("TACAN", info, nil)
}

func (e *elem) ndb() error {
	v := aixm45.NewNdb(e.n)
	f := newFeature("NDB", e.uuid)
	p := f.props
	info := e.equipment(f, v.Equipment)
	e.code(p, "emissionClass", v.CodeEm, "NDB-EM", "emission")
	e.measure(p, "frequency", v.Freq, v.FreqUom, "NDB-FREQ", ntFrequency, "uom_freq")
	e.code(p, "class", v.CodeClass, "NDB-CLASS", "ndb_class")
	e.timesheets("Ntt")
	if err := e.emit(f, orderNDB); err != nil {
		return err
	}
	return e.navaid("NDB", info, nil)
}

// verticalStructureTypes is the 5.1.1 CodeVerticalStructureType list
// (AIXM_DataTypes.xsd), for Config.ObstacleType results.
var verticalStructureTypes = map[string]bool{}

func init() {
	for _, s := range []string{"AG_EQUIP", "ANTENNA", "ARCH", "BRIDGE", "BUILDING", "CABLE_CAR", "CATENARY", "COMPRESSED_AIR_SYSTEM",
		"CONTROL_MONITORING_SYSTEM", "CONTROL_TOWER", "COOLING_TOWER", "CRANE", "DAM", "DOME", "ELECTRICAL_EXIT_LIGHT", "ELECTRICAL_SYSTEM",
		"ELEVATOR", "FENCE", "FUEL_SYSTEM", "GATE", "GENERAL_UTILITY", "GRAIN_ELEVATOR", "HEAT_COOL_SYSTEM", "INDUSTRIAL_SYSTEM", "LIGHTHOUSE",
		"MONUMENT", "NATURAL_GAS_SYSTEM", "NATURAL_HIGHPOINT", "NAVAID", "NUCLEAR_REACTOR", "POLE", "POWER_PLANT", "REFINERY", "RIG",
		"SALTWATER_SYSTEM", "SIGN", "SPIRE", "STACK", "STADIUM", "STORM_SYSTEM", "TANK", "TETHERED_BALLOON", "TOWER", "TRAMWAY",
		"TRANSMISSION_LINE", "TREE", "URBAN", "VEGETATION", "WALL", "WASTEWATER_SYSTEM", "WATER_SYSTEM", "WATER_TOWER", "WINDMILL",
		"WINDMILL_FARMS", "OTHER"} {
		verticalStructureTypes[s] = true
	}
}

func (e *elem) obs() error {
	v := aixm45.NewObs(e.n)
	f := newFeature("VerticalStructure", e.uuid)
	p := f.props
	e.str(p, "name", v.Name, "OBS-NAME", ttName)
	if is(v.DescrType) && v.DescrType.Text != "" {
		code := ""
		if e.r.cfg.ObstacleType != nil {
			code = e.r.cfg.ObstacleType(v.DescrType.Text)
		}
		if verticalStructureTypes[code] {
			p.add("type", el("type", code))
			e.markAs(v.DescrType, "OBS-TYPE", convert.Mapped, "")
		} else {
			p.add("type", el("type", "OTHER"))
			p.note("type", "DESCRIPTION", v.DescrType.Text)
			e.mark(v.DescrType, "OBS-TYPE")
		}
	}
	e.code(p, "lighted", v.CodeLgt, "OBS-LGT", "yes_no")
	e.code(p, "group", v.CodeGroup, "OBS-GROUP", "yes_no")
	e.note(p, "lighted", "DESCRIPTION", v.DescrLgt, "OBS-DESCRLGT")
	e.note(p, "markingICAOStandard", "DESCRIPTION", v.DescrMarking, "OBS-DESCRMARK")
	part := f.newProps()
	e.measure(part, "verticalExtent", v.Height, v.Elevation.Uom, "OBS-HGT", ntDistance, "uom_dist")
	if pp := e.position(f, v.Position, "OBS-POS"); pp != nil {
		e.measure(pp, "horizontalAccuracy", v.GeoAccuracy, v.GeoAccuracyUom, "GEO-HACC", ntDistance, "uom_dist")
		e.elevation(pp, v.Elevation)
		part.add("horizontalProjection_location", wrap("horizontalProjection_location", point(pp, true)))
	}
	p.add("part", wrap("part", part.object("VerticalStructurePart", "vsp", orderVSPart)))
	e.common(p, v.Common)
	return e.emit(f, orderVS)
}

func (e *elem) ahp() error {
	v := aixm45.NewAhp(e.n)
	f := newFeature("AirportHeliport", e.uuid)
	p := f.props
	e.str(p, "designator", v.CodeID, "AHP-DESIG", ttAhpDesig)
	e.str(p, "name", v.Name, "AHP-NAME", ttName)
	e.str(p, "locationIndicatorICAO", v.CodeIcao, "AHP-ICAO", ttICAO)
	e.str(p, "designatorIATA", v.CodeIata, "AHP-IATA", ttIATA)
	e.code(p, "type", v.CodeType, "AHP-TYPE", "ahp_type")
	e.measure(p, "fieldElevation", v.Elevation.Elev, v.Elevation.Uom, "AHP-ELEV", ntVertical, "uom_distver")
	e.measure(p, "fieldElevationAccuracy", v.Elevation.Accuracy, v.Elevation.Uom, "AHP-ELEVACC", ntVertical, "uom_distver")
	e.num(p, "magneticVariation", v.MagVar, "AHP-MAGVAR", ntMagVar)
	e.num(p, "dateMagneticVariation", v.MagVarDate, "AHP-MAGVARDATE", ntYear)
	e.num(p, "magneticVariationChange", v.MagVarChange, "AHP-MAGVARCHG", ntMagVar)
	e.measure(p, "referenceTemperature", v.RefTemp, v.RefTempUom, "AHP-REFT", ntTemp, "uom_temp")
	e.measure(p, "transitionAltitude", v.TransitionAlt, v.TransitionAltUom, "AHP-TA", ntVertical, "uom_distver")
	if is(v.NameCityServed) {
		cp := f.newProps()
		e.str(cp, "name", v.NameCityServed, "AHP-CITY", ttName)
		if len(cp.list) > 0 {
			p.add("servedCity", wrap("servedCity", cp.object("City", "city", orderCity)))
		}
	}
	if u, ok := e.ref(v.Org, "AHP-ORG"); ok {
		rp := f.newProps()
		rp.add("theOrganisationAuthority", href("theOrganisationAuthority", u))
		p.add("responsibleOrganisation", wrap("responsibleOrganisation", rp.object("AirportHeliportResponsibilityOrganisation", "resp", orderAhpResp)))
	}
	if pp := e.position(f, v.Position, "AHP-POS"); pp != nil {
		e.measure(pp, "horizontalAccuracy", v.GeoAccuracy, v.GeoAccuracyUom, "GEO-HACC", ntDistance, "uom_dist")
		e.measure(pp, "geoidUndulation", v.Elevation.GeoidUndulation, v.Elevation.Uom, "AHP-GEOID", ntSigned, "uom_distver")
		p.add("ARP", wrap("ARP", point(pp, true)))
	}
	e.mark(v.NameAdmin, "AHP-ADMIN")
	e.note(p, "ARP", "DESCRIPTION", v.DescrRefPoint, "AHP-DESCRREF")
	e.note(p, "", "DESCRIPTION", v.DescrSite, "AHP-DESCRSITE")
	e.note(p, "altimeterCheckLocation", "DESCRIPTION", v.DescrAltimeterCheck, "AHP-DESCRACL")
	e.timesheets("Aht")
	e.common(p, v.Common)
	return e.emit(f, orderAhp)
}

func (e *elem) rwy() error {
	v := aixm45.NewRwy(e.n)
	f := newFeature("Runway", e.uuid)
	p := f.props
	e.str(p, "designator", v.Desig, "RWY-DESIG", ttDesig)
	e.measure(p, "nominalLength", v.Len, v.DimUom, "RWY-LEN", ntDistance, "uom_dist")
	e.measure(p, "nominalWidth", v.Wid, v.DimUom, "RWY-WID", ntDistance, "uom_dist")
	e.measure(p, "lengthStrip", v.LenStrip, v.DimStripUom, "RWY-LENSTRIP", ntDistance, "uom_dist")
	e.measure(p, "widthStrip", v.WidStrip, v.DimStripUom, "RWY-WIDSTRIP", ntDistance, "uom_dist")
	sp := f.newProps()
	e.code(sp, "composition", v.Composition, "RWY-COMP", "surface")
	e.num(sp, "classPCN", v.PcnClass, "RWY-PCN", ntPCN)
	e.code(sp, "pavementTypePCN", v.PcnPavementType, "RWY-PCNTYPE", "pcn_pavement")
	e.code(sp, "pavementSubgradePCN", v.PcnSubgrade, "RWY-PCNSUB", "pcn_subgrade")
	e.code(sp, "maxTyrePressurePCN", v.PcnTyrePressure, "RWY-PCNTYRE", "pcn_tyre")
	e.code(sp, "evaluationMethodPCN", v.PcnEvalMethod, "RWY-PCNEVAL", "pcn_method")
	e.note(sp, "", "DESCRIPTION", v.PcnNote, "RWY-PCNNOTE")
	if len(sp.list) > 0 {
		p.add("surfaceProperties", wrap("surfaceProperties", sp.object("SurfaceCharacteristics", "sfc", orderSurface)))
	}
	if u, ok := e.ref(v.Ahp, "RWY-AHP"); ok {
		p.add("associatedAirportHeliport", href("associatedAirportHeliport", u))
	}
	e.note(p, "", "DESCRIPTION", v.Profile, "RWY-PROFILE")
	e.note(p, "", "DESCRIPTION", v.Marking, "RWY-MARKING")
	e.common(p, v.Common)
	return e.emit(f, orderRunway)
}

func (e *elem) rdn() error {
	v := aixm45.NewRdn(e.n)
	f := newFeature("RunwayDirection", e.uuid)
	p := f.props
	e.str(p, "designator", v.Desig, "RDN-DESIG", ttDesig)
	e.num(p, "trueBearing", v.TrueBrg, "RDN-TRUEBRG", ntBearing)
	e.num(p, "magneticBearing", v.MagBrg, "RDN-MAGBRG", ntBearing)
	e.measure(p, "elevationTDZ", v.ElevTdz, v.ElevTdzUom, "RDN-TDZ", ntVertical, "uom_distver")
	if u, ok := e.ref(v.Rwy, "RDN-RWY"); ok {
		p.add("usedRunway", href("usedRunway", u))
	}
	e.mark(v.Position.Lat, "RDN-POS")
	e.mark(v.Position.Long, "RDN-POS")
	e.mark(v.Position.Datum, "RDN-POS")
	e.common(p, v.Common)
	return e.emit(f, orderRdn)
}

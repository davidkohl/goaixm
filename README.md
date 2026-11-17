# goaixm

A Go library for parsing and working with [AIXM 5.1.1](https://aixm.aero)
(Aeronautical Information Exchange Model) data.

- **Complete generated model**: all 125 AIXM 5.1.1 features with their time
  slices, the 114 objects nested in them (airspace classes, segment points,
  procedure transitions, timesheets, notes, …) and 589 data types, generated
  from the official schemas. Units of measure and nil reasons are kept.
- **Parse or stream**: `Parse` indexes the whole message so references can be
  resolved; `Stream` visits one member at a time with memory bounded by the
  largest member.
- **Extensible**: replace a feature type, add overlay schemas decoded
  alongside the model, or keep unknown members as lossless element trees.
- **Time slices**: filter by interpretation and validity, pick the current
  state, merge BASELINE with PERMDELTA/TEMPDELTA.
- **Geometry**: RFC 7946 GeoJSON for airspaces with geodesic arcs and
  circles on WGS-84, referenced GeoBorders and contributor airspaces; helpers
  for every AIXM point, curve and surface.
- **EUROCONTROL ADR 23.5.0** extension, opt-in.
- **Version-aware**: the AIXM version of every document is detected; other
  versions are refused with `ErrUnsupportedVersion` or converted to 5.1.1
  through pluggable converters, with a report accounting for every value.
- No dependencies outside the standard library.

```sh
go get github.com/davidkohl/goaixm
```

## Quick start

```go
ctx := context.Background()
msg, err := goaixm.ParseFile(ctx, "data.xml", goaixm.Options{})
if err != nil {
	log.Fatal(err)
}
for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
	ts := a.TimeSlice[0]
	fmt.Println(a.UUID(), ts.Designator.String(), ts.Type.String())
	for _, c := range ts.Class {
		fmt.Println("  class", c.Classification.String())
	}
}
```

Data types are `aixm.Value` (text, `UOM`, `NilReason`); `String()` and
`Float()` are nil-safe. References to other features are `aixm.Reference`
values; follow them with the message:

```go
seg := goaixm.FeaturesOf[*aixm.RouteSegment](msg)[0]
start := seg.TimeSlice[0].Start.PointChoiceFixDesignatedPoint
if f, ok := msg.Resolve(start.Href); ok {
	fmt.Println(f.Kind(), f.(*aixm.DesignatedPoint).TimeSlice[0].Designator.String())
}
```

## Streaming

```go
err := goaixm.StreamFile(ctx, "big.xml", goaixm.Options{}, func(m goaixm.Member) error {
	if a, ok := m.Feature.(*aixm.Airspace); ok {
		fmt.Println(a.UUID())
	}
	return nil
})
```

Both functions accept `AIXMBasicMessage` and `ADRMessage` documents.

## Options

```go
opts := goaixm.Options{
	Registry: reg,                    // nil: all AIXM 5.1.1 features
	Fallback: goaixm.FallbackGeneric, // keep unknown members as element trees
	KeepRaw:  true,                   // keep each member's XML (Member.Raw, Member.Node)
	Strict:   false,                  // first problem becomes an error
	MaxBytes: 50 << 20,               // fail with ErrTooLarge beyond 50 MB
	OnWarning: func(w goaixm.Warning) error { log.Print(w); return nil },
}
```

Problems with single members are `Warning`s (also in `Message.Warnings`);
unknown kinds wrap `ErrUnknownFeature`.

## Extending the model

A `Registry` decides how members are decoded. See
[extensions/README.md](extensions/README.md) for details.

```go
reg := goaixm.NewDefaultRegistry()

// Overlay: a partial struct decoded alongside the model.
type myAirspace struct {
	Activations []string `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>AirspaceTimeSlice>activation>AirspaceActivation>status"`
}
reg.Extend("Airspace", func() any { return new(myAirspace) })

// Replace the type of a feature (it must implement aixm.Feature).
reg.Register("Unit", func() aixm.Feature { return new(MyUnit) })

msg, _ := goaixm.ParseFile(ctx, path, goaixm.Options{Registry: reg})
m, _ := msg.Member(airspace)
ext, ok := goaixm.Extension[*myAirspace](m)
```

## Time slices

Every time slice implements `aixm.TimeSlice`:

```go
slices := filter.Chain(
	filter.ByInterpretation(aixm.Baseline),
	filter.ByValidTime(time.Now()),
).Apply(airspace.TimeSlices())

ts, err := filter.Current(airspace.TimeSlices(), time.Now()) // SNAPSHOT, or BASELINE+deltas
merged, err := filter.EffectiveAt(airspace.TimeSlices(), time.Now())
a := merged.(*aixm.AirspaceTimeSlice)
```

## GeoJSON

```go
fc, err := geo.AirspaceToGeoJSONWithRefs(airspace, msg)
data, _ := geo.MarshalGeoJSON(fc)
```

Pass the message: geometry that lives elsewhere in the document, such as a
boundary shared through a `GeoBorder`, a sub-airspace contributed via
`contributorAirspace` or an arc centre held in another feature, can only be
resolved with it. `geo.AirspaceComponents` returns the components unmerged,
including `INTERS`/`SUBTR`, for evaluation elsewhere (e.g. PostGIS). For other
geometry use `geo.FromPoint`, `geo.FromElevatedPoint`, `geo.CurveCoords`,
`geo.SegmentCoords` and `geo.SurfacePolygons`.

| GML construct | Handling |
|---|---|
| `LinearRing`, `GeodesicString`, `LineStringSegment` | Vertices taken as given (densified with `geo.Options`, below) |
| `CircleByCenterPoint`, `ArcByCenterPoint` | Discretised geodesically on WGS-84 (Vincenty direct); point count scales with the swept angle |
| `Arc`, `Circle` (three-point forms) | Centre found planar, then walked geodesically |
| `curveMember` with `xlink:href` | Resolved to the referenced `GeoBorder` and cut where the neighbouring members attach (a neighbouring border: where the two meet), so whole borders are joined end to end and reversed as needed; consecutive references to one border count as one stretch; gaps are reported in `AirspaceComponent.Warnings` |
| `aixm:Curve` / `aixm:Point` in a GML slot | Accepted wherever they substitute for the GML element |
| Mixed segments in one curve | Assembled in document order |

Radii are converted from their unit of measure (`NM`, `KM`, `M`, `FT`, `MI`,
`CM` and the UCUM spellings including `[nmi_i]`). An unrecognised unit is an
error rather than an assumed default. Rings are closed and wound per
RFC 7946. `geo.DefaultArcPoints` (64) sets the points of a full circle.

By default arcs use a fixed number of points and geodesic segments keep only
their vertices: at 64 points a 10 NM circle is off by up to 22 m, and a
200 km east-west `GeodesicString` drawn straight in lon/lat by up to 1 km.
`geo.Options` densifies by a maximum error instead; every source vertex is
kept exactly, computed positions are only inserted between them:

```go
opt := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1} // metres
comps, err := opt.AirspaceComponents(airspace, msg)
```

| Option | Applies to | Rule |
|---|---|---|
| `ArcTolerance` | `ArcByCenterPoint`, `CircleByCenterPoint`, `Arc`, `Circle` | Equal bearing steps with chord sagitta r·(1 − cos(Δθ/2)) ≤ tolerance, then verified per chord against the straight lon/lat line (more steps if needed); at least 8 per arc |
| `GeodesicTolerance` | `GeodesicString`, `LineStringSegment interpolation="geodesic"` (also inside `GeoBorder`s) | Each segment split into n equal geodesic pieces (Vincenty direct), n the smallest found with every piece's lon/lat chord within the tolerance at its midpoint; segments crossing the antimeridian are left as they are |

| `PreferSourceVertices` | joins of a computed arc end with an explicit vertex | The explicit source position is kept |
| `BorderSnapTolerance` | rings borrowing a referenced `GeoBorder` | An explicit end vertex of the neighbouring curve that lies beyond the border (its own last or first piece crosses it) and within the tolerance is moved onto the border at the cut point; each move is reported to `OnRepair` as a `geo.Repair` |

`geo.Options{NestedContributors: true}` gives a contributor airspace that is
itself an aggregate as its own components (`AirspaceComponent.Contributor`,
with their operations, recursively), so that callers combine each level;
without it, its polygons are packed together unmerged.

`geo.BorderStretch` exposes the border cut on its own (with an option to
take the shorter way round a closed border), `geo.RhumbLine` densifies a
rhumb line (WGS-84 loxodrome) into geodesic pieces within a tolerance.

`geo.DensifyVersion` changes whenever the same options give different
positions, for callers that store converted geometry.

The `Options` methods mirror the package functions (`SegmentCoords`,
`CurveCoords`, `SurfaceRings`, `SurfacePolygons`, `AirspaceComponents`,
`AirspaceToGeoJSON`, `AirspaceToGeoJSONWithRefs`); the zero value behaves
like them.

## AIXM versions and conversion

Package `version` detects the AIXM version of a document from its root
element (`AIXM-Snapshot version="4.5"`, or the AIXM namespace of
`AIXMBasicMessage`/`ADRMessage`):

```go
d, err := version.DetectFile("data.xml") // d.Version, d.Root, d.Attrs
```

`Parse` and `Stream` read AIXM 5.1.1. A document of another AIXM version
fails with `goaixm.ErrUnsupportedVersion`, unless `Options.Converters` holds
a path of converters to 5.1.1: then it is converted on the fly and its
records are decoded like native members.

```go
reg := convert.NewRegistry(myConverter) // any convert.Converter
err := goaixm.StreamFile(ctx, "old.xml", goaixm.Options{
	Converters:  reg,
	OnConverted: func(r *convert.Report) error { report = r; return nil },
}, func(m goaixm.Member) error {
	// m.Raw is the converted 5.1.1 XML, m.Provenance its source element(s)
	return nil
})
```

Package `convert` is the framework; it ships no concrete converter:

| Type | Role |
|---|---|
| `Converter` | `From`, `To`, `ID` and `Convert(ctx, Source, Sink)`: writes the target document one standalone member element (`Record`) at a time, deterministically |
| `Source` | A re-openable document (`FileSource`, `BytesSource`, `Spool` for a one-shot reader), for converters that read twice |
| `Sink` | Receives `Begin(Header)`, `Put(Record)`, `End()`; `XMLWriter` writes an `AIXMBasicMessage`, `Tee` fans out, `goaixm.MemberSink` decodes |
| `Registry` | One converter per version pair; `Path` finds the shortest chain, `Converter` returns it composed with `Chain` |
| `Report` | Per source kind and path: counts by `Outcome` (`mapped`, `derived`, `defaulted`, `lossy`, `unmapped`, `invalid`), samples, issues, source and output sha256, `Digest` |
| `ConverterID` | Name, rule version, mapping-table digest and module version of the rules used |

`convert.Run` drives a converter: it checks the source version and the sink
call order, completes the report and verifies its accounting
(`Report.Check`): every source leaf (attribute or element text, see
`convert.WalkLeaves`) gets exactly one outcome, and every source element is
emitted, merged or dropped. Equal input gives byte-identical output and an
equal report digest. `convert.NewV5` derives stable UUIDs for converted
identities.

`ParseSource`/`StreamSource` take a `convert.Source`; `Parse`/`Stream` with a
plain reader copy a document that needs conversion to `Options.SpoolDir`
first. Native 5.1.1 input is never copied and decodes exactly as without
converters.

### AIXM 4.5

Package `aixm45` reads AIXM 4.5 snapshots: `Stream`/`Reader` yield one
top-level element at a time as an `xmlnode` tree, `Key` gives the canonical
natural key of a `…Uid` element (`Rdn(RwyUid(AhpUid(codeId=XA01);txtDesig=09/27);txtDesig=27)`),
`ParseLat`/`ParseLong` parse the 4.5 coordinate layouts exactly, and typed
views (`NewAhp`, `NewDpn`, `NewVor`, …) name the values of the common
entities while keeping their source nodes.

Package `convert/aixm45to51` is a `convert.Converter` from 4.5 to 5.1.1:

```go
reg := convert.NewRegistry(aixm45to51.New(aixm45to51.Config{
	Namespace: myNamespace, // one fixed UUID per data originator
}))
msg, err := goaixm.ParseFile(ctx, "snapshot45.xml", goaixm.Options{Converters: reg})
```

- Coverage: `Org`, `Uni`, `Dpn`, `Vor`/`Dme`/`Ndb`/`Tcn` (plus the `Navaid`
  derived from `VorUid` collocation), `Obs`, `Ahp`, `Rwy`, `Rdn`; `Ase`
  with its boundary (`Abd`) or derivation (`Adg`) and working hours (`Att`)
  as one `Airspace`, `Gbr` as `GeoBorder`. All other kinds are reported as
  unmapped and dropped.
- Airspace geometry: great circles as `GeodesicString`, rhumb lines
  densified on the loxodrome (`Config.RhumbTolerance`, 1 m), arcs as
  `ArcByCenterPoint` in geo's signed-sweep convention, circles as
  `CircleByCenterPoint`, border-following edges cut with
  `geo.BorderStretch` and written inline (default, self-contained
  airspaces) or as a `GeoBorder` reference (`Config.BorderMode`). Derived
  airspaces become `BASE`/`UNION`/… components with `contributorAirspace`.
  Rings that enclose no area keep their positions in a note.
- Identity: UUIDv5 over the natural key in the caller's namespace;
  references are computed from the referencing `…Uid`; duplicate keys get
  `#2`, `#3`, … and a `duplicate_key` warning.
- One BASELINE time slice per feature from the snapshot's `effective`.
- Coordinates on another datum than WGS-84 are carried untransformed,
  flagged with a WARNING note and reported per feature.
- The rules are data: `mapping.tsv` (rule id, source, target, outcome,
  reference) and `codes.tsv`, hashed into `ConverterID.MappingDigest`. They
  are goaixm's own proposal ("goaixm" in the reference column) until
  reconciled with the official EUROCONTROL 4.5 to 5.1 mapping.
- Tests check every emitted code and unit against the vendored 5.1.1
  schema, the property order against its sequences, and a golden report
  (`testdata/aixm45/*.report.json`, `go test -update`).

## Validation

```go
for _, e := range aixm.ValidateAll(msg.Features()) {
	fmt.Println(e)
}
```

## EUROCONTROL ADR

```go
reg := goaixm.NewDefaultRegistry()
adr.Register(reg)
msg, _ := goaixm.ParseFile(ctx, "adr.xml", goaixm.Options{Registry: reg})

for _, e := range airport.TimeSlice[0].Extension {
	if ext, ok := adr.DecodeExtension[adr.AirportHeliportExtension](e); ok {
		fmt.Println(ext.NMDesignator.String())
	}
}
```

## Layout

| Path | Contents |
|---|---|
| `.` (package `goaixm`) | `Parse`, `Stream`, `ParseSource`, `StreamSource`, `Registry`, `Message`, `Member`, `MemberSink` |
| `version/` | AIXM versions and document version detection |
| `convert/` | Converter interface, registry, chains, reports, `XMLWriter` |
| `convert/aixm45to51/` | The AIXM 4.5 to 5.1.1 converter and its mapping tables |
| `aixm45/` | AIXM 4.5 reader, natural keys, coordinates, typed views |
| `aixm/` | The AIXM 5.1.1 model; `*_gen.go` are generated |
| `gml/` | The GML 3.2.1 subset AIXM uses |
| `geo/` | Coordinates, geodesic arcs, GeoJSON |
| `filter/` | Time slice selection and merging |
| `xmlnode/` | Lossless, schema-free XML element trees |
| `extensions/adr/` | EUROCONTROL ADR 23.5.0 |
| `tools/` | The model generator and the AIXM schemas (not imported) |
| `testdata/` | Fixtures and the DONLON reference data (git submodule) |

## Development

```sh
git submodule update --init   # DONLON reference data
go generate ./...             # regenerate aixm/*_gen.go from tools/xsd
go vet ./... && go test ./...
go test -fuzz=FuzzParse -fuzztime=30s .
go test -fuzz=FuzzDetect -fuzztime=30s ./version
go test -fuzz=FuzzStream -fuzztime=30s ./aixm45
go test -fuzz=FuzzCoordinate -fuzztime=30s ./aixm45
go test -fuzz=FuzzConvert -fuzztime=30s ./convert/aixm45to51
```

`internal/testconv` holds a namespace-relabelling converter used only by
the tests to exercise the conversion framework; it is not a real version
conversion.

Tests use the EUROCONTROL [DONLON](https://github.com/aixm/donlon)
reference data and skip when the submodule is missing. The AIXM 4.5 tests
use small synthetic, hand-written fixtures in `testdata/aixm45/` (fictional
designators and coordinates). CI also checks gofmt,
staticcheck and that the generated code is up to date.

## Limitations

- **No schema validation** at parse time: members that cannot be decoded are
  reported as warnings, not schema errors.
- **Corridor airspaces** (a `centreline` with a `width`) are returned as a
  `LineString` with a `width_m` property, not buffered.
- **Airspace aggregation** in `AirspaceToGeoJSON*` supports `BASE` and
  `UNION`; `INTERS` and `SUBTR` are rejected rather than approximated (use
  `AirspaceComponents`).
- **No CRS reprojection**: coordinates are taken as EPSG:4326
  latitude/longitude.
- **No DO-178C / DO-200B certification.** This library has not been through
  a formal avionics software or data quality assurance process and must not
  be used as a safety-critical component in operational ATM systems without
  independent certification.

## Namespaces

| Prefix | URI |
|---|---|
| `aixm` | `http://www.aixm.aero/schema/5.1.1` |
| `message` | `http://www.aixm.aero/schema/5.1.1/message` |
| `adr` | `http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR` |
| `gml` | `http://www.opengis.net/gml/3.2` |
| `xlink` | `http://www.w3.org/1999/xlink` |

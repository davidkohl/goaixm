# goaixm

A Go library for parsing and working with [AIXM 5.1.1](https://aixm.aero) (Aeronautical Information Exchange Model) data.

## Features

- **Full AIXM 5.1.1 type coverage** — 127 feature types generated from the official XSD schemas
- **Two-pass XML decoder** — handles polymorphic `hasMember` dispatch correctly
- **Streaming parser** — O(largest member) memory, suitable for multi-gigabyte NM B2B files
- **TimeSlice filtering** — filter by interpretation, valid time, or compute the effective state at any point in time (BASELINE + PERMDELTA + TEMPDELTA merging)
- **XLink resolution** — automatically resolves `xlink:href` cross-references after decode
- **GeoJSON export** — converts airspace geometry to GeoJSON including interior rings (holes), arc/circle discretization, and referenced sub-airspace assembly
- **Feature validation** — explicit data quality checks per feature type
- **EUROCONTROL ADR 23.5.0 extension** — opt-in via blank import
- **Fuzz-tested** — `go test -fuzz` harness with 600k+ iterations, no panics

## Installation

```sh
go get gitlab.com/davidkohl/goaixm
```

## Quick start

```go
msg, err := goaixm.ParseFile("data.xml")
if err != nil {
    log.Fatal(err)
}

for _, a := range msg.Airspaces() {
    if len(a.TimeSlice) == 0 {
        continue
    }
    ts := a.TimeSlice[0].AirspaceTimeSlice
    if ts.Designator != nil {
        fmt.Println(ts.Designator.Value)
    }
}
```

## Streaming large files

```go
err := goaixm.StreamFile(ctx, "big.xml", func(localName string, feat any) error {
    if localName == "Airspace" {
        a := feat.(*features.AirspaceType)
        fmt.Println(a.GetIdentifier())
    }
    return nil
})
```

## TimeSlice filtering

```go
slices := filter.Chain(
    filter.ByInterpretation(features.InterpretationBaseline),
    filter.ByValidTime(time.Now()),
).Apply(airspace.TimeSlices())
```

## Effective state (BASELINE + deltas)

```go
state, err := filter.EffectiveAt(airspace.TimeSlices(), time.Now())
if err != nil {
    log.Fatal(err) // no BASELINE valid at this time
}
ts := state.Slice.(*features.AirspaceTimeSliceType)
fmt.Println(ts.Designator.Value)
```

## XLink resolution

```go
feat, ok := msg.Resolve("urn:uuid:some-uuid-here")
```

## GeoJSON export

```go
fc, err := geo.AirspaceToGeoJSON(airspace)
if err != nil {
    log.Fatal(err)
}
data, _ := geo.MarshalGeoJSON(fc)
os.WriteFile("airspace.geojson", data, 0644)
```

For airspaces that reference sub-airspace geometry via `xlink:href` (e.g. EDDH-style):

```go
fc, err := geo.AirspaceToGeoJSONWithRefs(airspace, msg)
```

## Feature validation

```go
var feats []features.Feature
for _, v := range msg.Features() {
    if f, ok := v.(features.Feature); ok {
        feats = append(feats, f)
    }
}
for _, e := range features.ValidateAll(feats) {
    fmt.Println(e)
}
```

## Parse options (size limit, strict mode)

```go
opts := parse.ParseOptions{
    MaxBytes:   50 * 1024 * 1024, // reject messages over 50 MB
    StrictMode: false,            // collect warnings instead of erroring
}
msg, err := parse.ParseMessageWithOptions(ctx, r, opts)
fmt.Printf("warnings: %d\n", len(msg.Warnings()))
```

## EUROCONTROL ADR extension

```go
import _ "gitlab.com/davidkohl/goaixm/extensions/adr"

msg, _ := goaixm.ParseFile("adr_message.xml")
// ADR feature types are now registered and decoded automatically.
```

## Custom type support

Feature decoding is driven by a registry mapping XML local names to factory
functions. To support feature types goaixm doesn't ship (e.g. a national
AIXM extension), define the struct and register it before parsing —
typically from an `init()` in your own package, mirroring `extensions/adr`:

```go
type ObstacleAreaType struct {
    XMLName   xml.Name `xml:"urn:example:ext ObstacleArea"`
    ID        string   `xml:"http://www.opengis.net/gml/3.2 id,attr"`
    TimeSlice []ObstacleAreaTimeSliceType `xml:"urn:example:ext timeSlice>ObstacleAreaTimeSlice"`
}

func init() {
    parse.DefaultRegistry.Register("ObstacleArea", func() any {
        return new(ObstacleAreaType)
    })
}
```

Registered types are decoded automatically wherever they appear as message
members and are returned by `msg.ByType("ObstacleArea")` and
`msg.Features()`. Extension elements inside existing feature timeslices
(the AIXM `extension` slots) need no registration — declare a struct whose
`XMLName` matches the extension element, as in
[`extensions/adr/adr.go`](extensions/adr/adr.go).

## Package structure

| Package | Purpose |
|---|---|
| `goaixm` | Top-level facade: `ParseFile`, `ParseFileWithOptions`, `StreamFile` |
| `parse` | Two-pass XML decoder, feature registry, streaming decoder |
| `features` | Generated AIXM feature structs + interfaces, validation |
| `filter` | TimeSlice filtering and effective-state merging |
| `geo` | Geometry extraction, WKT, GeoJSON export |
| `gml` | Hand-crafted GML 3.2.1 types |
| `types` | Generated AIXM data types |
| `message` | AIXMBasicMessage and ADRMessage envelope types |
| `extensions/adr` | EUROCONTROL ADR 23.5.0 extension (opt-in) |
| `internal/cmd/fixgen` | Code generation post-processor |

## Code generation

Feature and data types are generated from the XSD files in `aixm-5.1.1/` using [`xgen`](https://github.com/xuri/xgen) followed by the `fixgen` post-processor (adds XML namespaces, XLink fields, nillable handling).

```sh
go install github.com/xuri/xgen/cmd/xgen@latest
go generate ./...
go build ./...
go test ./...
```

Generated files (`features/features.go`, `types/datatypes.go`, `message/message.go`) are build artefacts — do not hand-edit them.

> **Note:** The ISO 19139 schemas (`aixm-5.1.1/iso/`) are excluded from the repository due to unclear redistribution rights (© ISO). Before running `go generate`, download them from [https://schemas.opengis.net/iso/19139/20070417/](https://schemas.opengis.net/iso/19139/20070417/) and place them at `aixm-5.1.1/iso/20070417/`.

## Namespaces

| Prefix | URI |
|---|---|
| `aixm` | `http://www.aixm.aero/schema/5.1.1` |
| `message` | `http://www.aixm.aero/schema/5.1.1/message` |
| `adr` | `http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR` |
| `gml` | `http://www.opengis.net/gml/3.2` |
| `xlink` | `http://www.w3.org/1999/xlink` |

## Testing

```sh
go test ./...
go test -fuzz=FuzzParseMessage -fuzztime=30s ./parse/...
go vet ./...
```

The test suite validates against the official [DONLON](https://ext.eurocontrol.int/aixm_confluence/display/DONLON) reference dataset (147 features, all AIXM 5.1.1 feature types represented).

## Limitations

- **No XML Schema validation** against the AIXM XSDs at parse time — malformed input is skipped with a `ParseWarning`, not rejected with a schema error.
- **Arc discretization** uses planar (Cartesian) geometry, not geodesic. For small airspace boundaries this is acceptable; for large arcs spanning many degrees of latitude a geodesic library should be used instead.
- **No DO-178C / DO-200B certification** — this library has not been through a formal avionics software or data quality assurance process. It must not be used as a safety-critical component in operational ATM systems without independent certification.

## License

See [LICENSE](LICENSE).

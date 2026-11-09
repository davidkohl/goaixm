# Extending goaixm

The generated AIXM types do not cover every element: many inline objects
(`AirspaceLayerClass`, `EnRouteSegmentPoint`, `ProcedureTransition`, …) are
xlink-only stubs. goaixm offers three ways to get the data you need without
forking the library.

All three work through a `*parse.Registry`. Start from the built-in types with
`parse.DefaultRegistry.Clone()`, or from nothing with `parse.NewRegistry()`, and
pass it via `parse.ParseOptions.Registry` or `parse.StreamOptions.Registry`.
Registries are independent of each other and safe for concurrent use.

## 1. Overlay schemas — `Registry.Extend`

An overlay is your own struct, decoded from the same member XML **in addition
to** the primary type. Declare only the elements you need; everything else is
ignored.

```go
type AirspaceClass struct {
    XMLName   xml.Name `xml:"http://www.aixm.aero/schema/5.1.1 Airspace"`
    TimeSlice []struct {
        TS struct {
            Class []struct {
                Classification string `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceLayerClass>classification"`
            } `xml:"http://www.aixm.aero/schema/5.1.1 class"`
        } `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceTimeSlice"`
    } `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice"`
}

reg := parse.DefaultRegistry.Clone()
reg.Extend("Airspace", func() any { return new(AirspaceClass) })

msg, _ := parse.ParseMessageWithOptions(ctx, r, parse.ParseOptions{Registry: reg})
for _, a := range msg.Airspaces() {
    m, _ := msg.Member(a)
    cls, ok := parse.Extension[*AirspaceClass](m)
    _ = cls; _ = ok
}
```

Overlays can reuse goaixm's geometry types, for example
`*features.CurvePropertyType`. Convert them with `geo.CurveCoords`,
`geo.SurfacePolygons` or `geo.FromPointType`.

## 2. Override — `Registry.Register`

`Register` replaces the primary type for a local name. To be indexed by
`Message.Feature` and `Message.Resolve`, your type should implement
`features.Feature` (`GetGMLID`, `GetIdentifier`, `GetNames`).

## 3. Lossless fallback — `xmlnode`

- `ParseOptions{Fallback: parse.FallbackGeneric}` decodes members with no
  registered type into `*parse.GenericFeature`, which holds an `*xmlnode.Node`
  tree. They are not dropped.
- `KeepRaw: true` keeps every member's XML; `Member.Node()` returns its tree.
- `xmlnode.Node` supports path lookups (`Find`, `TextAt`, `Href`), decoding
  into any struct (`Decode`) and lossless JSON.

## Streaming

`parse.StreamMessageWithOptions` delivers `parse.Member` values (primary type,
overlays and optional raw XML) with bounded memory. Cross-feature xlinks are
not resolved while streaming.

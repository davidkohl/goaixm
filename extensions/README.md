# Extending goaixm

The generated model in package `aixm` covers the AIXM 5.1.1 schemas. For
national extensions, profile-specific data or anything else it does not
cover, goaixm offers three mechanisms. All of them work through a
`*goaixm.Registry`, passed in `goaixm.Options.Registry`. Start from
`goaixm.NewDefaultRegistry()` (all AIXM features) or `goaixm.NewRegistry()`
(empty). Registries are independent of each other and safe for concurrent
use.

## 1. Overlay schemas: `Registry.Extend`

An overlay is your own struct, decoded from the same member XML **in
addition to** the primary type. Declare only the elements you need, using
`a>b` paths through the time slice:

```go
type AirspaceActivation struct {
	Status []string `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>AirspaceTimeSlice>activation>AirspaceActivation>status"`
}

reg := goaixm.NewDefaultRegistry()
reg.Extend("Airspace", func() any { return new(AirspaceActivation) })

msg, _ := goaixm.ParseFile(ctx, path, goaixm.Options{Registry: reg})
for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
	m, _ := msg.Member(a)
	act, ok := goaixm.Extension[*AirspaceActivation](m)
	_, _ = act, ok
}
```

Overlays can use the model's own types, for example `*aixm.CurveProperty`,
and convert them with package `geo`.

## 2. Replacing a feature type: `Registry.Register`

`Register` sets the primary type for a feature kind. The type must implement
`aixm.Feature`; embedding `aixm.FeatureCommon` provides `Common()` and the
identity fields:

```go
type MyUnit struct {
	XMLName xml.Name `xml:"http://www.aixm.aero/schema/5.1.1 Unit"`
	aixm.FeatureCommon
	Names []string `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>UnitTimeSlice>name"`
}

func (*MyUnit) Kind() string                  { return "Unit" }
func (*MyUnit) TimeSlices() []aixm.TimeSlice { return nil }

reg.Register("Unit", func() aixm.Feature { return new(MyUnit) })
```

New feature kinds from an extension namespace are registered the same way;
see package `extensions/adr`.

## 3. Lossless fallback: `xmlnode`

- `Options{Fallback: goaixm.FallbackGeneric}` decodes members without a
  registered type into `*goaixm.GenericFeature`, which holds an
  `*xmlnode.Node` tree, instead of dropping them.
- `Options{KeepRaw: true}` keeps every member's XML; `Member.Node()` returns
  its tree.
- `xmlnode.Node` supports path lookups (`Find`, `TextAt`, `Href`), decoding a
  subtree into any struct (`Decode`) and a lossless JSON encoding.

## Extension slots

AIXM time slices and objects carry `extension` elements, kept raw as
`[]aixm.Extension`. Decode them with `xml.Unmarshal(ext.Inner, &v)`, or with
a helper such as `adr.DecodeExtension`.

## Streaming

`goaixm.Stream` delivers `goaixm.Member` values (primary type, overlays and
optional raw XML) with the same options and bounded memory. Cross-feature
references are not resolved while streaming.

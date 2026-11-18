# tools

Development tools; nothing here is imported by the library.

- `aixmgen/` generates the model of package `aixm` (`*_gen.go`) from the
  AIXM 5.1.1 schemas. It runs through `go generate ./...`, which CI checks
  produces no diff.
- `xsd/aixm-5.1.1/` holds the AIXM 5.1.1 schemas and the GML 3.2.1 and message
  schemas they import. The ISO 19139 schemas referenced by the metadata types
  are not redistributed; aixmgen does not need them. For schema validation,
  download them from <https://schemas.opengis.net/iso/19139/20070417/> into
  `xsd/aixm-5.1.1/iso/`.

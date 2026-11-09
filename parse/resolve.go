package parse

import (
	"reflect"
	"regexp"
	"strings"
)

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// resolveXLinks walks every decoded feature and fills nil inline pointers on
// property types that carry a non-empty xlink:href attribute.  It uses the
// byID index that was already built during the second decode pass.
//
// The AIXM property-type pattern is:
//
//	type FooPropertyType struct {
//	    Foo      *FooType `xml:"..."`
//	    HrefAttr string   `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
//	    ...
//	}
//
// When a feature is encoded by reference the Foo field is nil and HrefAttr
// holds a URI like "urn:uuid:...".  resolveXLinks finds those fields and sets
// Foo to the matching decoded feature pointer.
func resolveXLinks(decoded []any, byID map[string]any) {
	for _, feat := range decoded {
		v := reflect.ValueOf(feat)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			continue
		}
		walkStruct(v, byID, 0)
	}
}

const maxDepth = 20 // guard against cyclic structures

func walkStruct(v reflect.Value, byID map[string]any, depth int) {
	if depth > maxDepth {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		ft := t.Field(i)

		// Skip unexported fields.
		if !ft.IsExported() {
			continue
		}

		// Dereference a settable pointer field.
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				continue
			}
			fv = fv.Elem()
		}

		if fv.Kind() == reflect.Slice {
			for j := 0; j < fv.Len(); j++ {
				elem := fv.Index(j)
				if elem.Kind() == reflect.Pointer {
					if elem.IsNil() {
						continue
					}
					elem = elem.Elem()
				}
				if elem.Kind() == reflect.Struct {
					tryResolve(elem, byID)
					walkStruct(elem, byID, depth+1)
				}
			}
			continue
		}

		if fv.Kind() == reflect.Struct {
			tryResolve(fv, byID, )
			walkStruct(fv, byID, depth+1)
		}
	}
}

// tryResolve checks whether v is a property type with HrefAttr set and a nil
// inline feature pointer, and if so resolves the reference.
func tryResolve(v reflect.Value, byID map[string]any) {
	hrefField := v.FieldByName("HrefAttr")
	if !hrefField.IsValid() || hrefField.Kind() != reflect.String {
		return
	}
	href := hrefField.String()
	if href == "" {
		return
	}
	id := hrefToID(href)
	if id == "" {
		return
	}
	target, ok := byID[id]
	if !ok {
		return
	}
	targetVal := reflect.ValueOf(target) // *ConcreteType

	// Look for the first exported pointer field whose type matches the target.
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		ft := t.Field(i)
		if !ft.IsExported() {
			continue
		}
		if fv.Kind() != reflect.Pointer {
			continue
		}
		if !fv.IsNil() {
			continue // already populated
		}
		if fv.Type() == targetVal.Type() {
			fv.Set(targetVal)
			return
		}
	}
}

// hrefToID extracts the UUID from an xlink:href value.
// AIXM uses two common forms:
//
//	urn:uuid:xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//	#xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func hrefToID(href string) string {
	// XPointer form: #xpointer(//*[@gml:identifier='<uuid>']) or similar.
	if strings.Contains(href, "xpointer(") {
		if m := uuidRe.FindString(href); m != "" {
			return m
		}
		return ""
	}
	if strings.HasPrefix(href, "urn:uuid:") {
		return strings.TrimPrefix(href, "urn:uuid:")
	}
	if strings.HasPrefix(href, "#") {
		return strings.TrimPrefix(href, "#")
	}
	// Plain UUID
	if len(href) == 36 && strings.Count(href, "-") == 4 {
		return href
	}
	return ""
}

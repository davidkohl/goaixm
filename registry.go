package goaixm

import (
	"sort"
	"sync"

	"github.com/davidkohl/goaixm/aixm"
)

// Registry decides how the members of a message are decoded.  It is the
// extension point of the parser:
//
//   - Register sets the primary type decoded for a feature kind, replacing
//     the AIXM 5.1.1 type when there is one.
//   - Extend adds an overlay schema: a caller-defined struct decoded from the
//     same member XML in addition to the primary type.  Overlays declare only
//     the elements they need; retrieve them with Extension.
//
// A Registry is safe for concurrent use.  Registries are independent: start
// from NewDefaultRegistry or NewRegistry and pass the result in Options.
type Registry struct {
	mu         sync.RWMutex
	factories  map[string]func() aixm.Feature
	extensions map[string][]func() any
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: map[string]func() aixm.Feature{}, extensions: map[string][]func() any{}}
}

// NewDefaultRegistry returns a registry holding every AIXM 5.1.1 feature
// type of package aixm.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	for _, kind := range aixm.FeatureKinds() {
		kind := kind
		r.factories[kind] = func() aixm.Feature {
			f, _ := aixm.NewFeature(kind)
			return f
		}
	}
	return r
}

// defaultRegistry is used when Options.Registry is nil.  It is never
// modified after initialisation.
var defaultRegistry = NewDefaultRegistry()

// Clone returns an independent copy of r, including its overlays.
func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c := NewRegistry()
	for k, v := range r.factories {
		c.factories[k] = v
	}
	for k, v := range r.extensions {
		c.extensions[k] = append([]func() any(nil), v...)
	}
	return c
}

// Register sets the primary type for a feature kind (XML local name).  f
// must return a pointer to a fresh value with encoding/xml tags.
func (r *Registry) Register(kind string, f func() aixm.Feature) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[kind] = f
}

// Extend adds an overlay schema for a feature kind.  Every member of that
// kind is additionally decoded into a fresh value from f.  Several overlays
// may be registered for one kind; they are kept in registration order.
func (r *Registry) Extend(kind string, f func() any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extensions[kind] = append(r.extensions[kind], f)
}

// Has reports whether a primary type is registered for kind.
func (r *Registry) Has(kind string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[kind]
	return ok
}

// Kinds returns the feature kinds with a registered primary type, sorted.
func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.factories))
	for k := range r.factories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) factory(kind string) (func() aixm.Feature, []func() any) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.factories[kind], r.extensions[kind]
}

package convert

import (
	"fmt"
	"sort"
	"sync"

	"github.com/davidkohl/goaixm/version"
)

// Registry holds converters, at most one per From/To pair, and finds
// chains between versions.  It is safe for concurrent use.
type Registry struct {
	mu sync.RWMutex
	cs map[[2]version.Version]Converter
}

// NewRegistry returns a registry holding cs.  It panics when Add would
// fail (a nil converter, From == To, or two converters for one pair): the
// set of converters is fixed program configuration.
func NewRegistry(cs ...Converter) *Registry {
	r := &Registry{cs: map[[2]version.Version]Converter{}}
	for _, c := range cs {
		if err := r.Add(c); err != nil {
			panic(err)
		}
	}
	return r
}

// Add registers c.  It fails for a nil converter, From == To, and a pair
// that already has a converter.
func (r *Registry) Add(c Converter) error {
	if c == nil {
		return fmt.Errorf("convert: nil converter")
	}
	from, to := c.From(), c.To()
	if from == to {
		return fmt.Errorf("convert: converter %s converts %v to itself", c.ID().Name, from)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cs == nil {
		r.cs = map[[2]version.Version]Converter{}
	}
	key := [2]version.Version{from, to}
	if old, ok := r.cs[key]; ok {
		return fmt.Errorf("convert: %v to %v already registered (%s)", from, to, old.ID().Name)
	}
	r.cs[key] = c
	return nil
}

// Converters returns the registered converters sorted by From, then To.
func (r *Registry) Converters() []Converter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Converter, 0, len(r.cs))
	for _, c := range r.cs {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if c := out[i].From().Compare(out[j].From()); c != 0 {
			return c < 0
		}
		return out[i].To().Less(out[j].To())
	})
	return out
}

// Path returns the shortest sequence of converters from one version to
// another; among equally short paths the one through the lowest
// intermediate versions wins, so the result is deterministic.  from == to
// gives an empty path.  No path gives an error wrapping ErrNoPath.
func (r *Registry) Path(from, to version.Version) ([]Converter, error) {
	if from == to {
		return nil, nil
	}
	all := r.Converters() // sorted: BFS visits neighbours in version order
	next := map[version.Version][]Converter{}
	for _, c := range all {
		next[c.From()] = append(next[c.From()], c)
	}
	via := map[version.Version]Converter{}
	seen := map[version.Version]bool{from: true}
	queue := []version.Version{from}
	for len(queue) > 0 && !seen[to] {
		v := queue[0]
		queue = queue[1:]
		for _, c := range next[v] {
			if !seen[c.To()] {
				seen[c.To()] = true
				via[c.To()] = c
				queue = append(queue, c.To())
			}
		}
	}
	if !seen[to] {
		return nil, fmt.Errorf("%w from AIXM %v to %v", ErrNoPath, from, to)
	}
	var path []Converter
	for v := to; v != from; v = via[v].From() {
		path = append([]Converter{via[v]}, path...)
	}
	return path, nil
}

// Converter returns one converter from one version to another: the
// registered converter, or a Chain over Path.  from == to is an error
// (nothing to convert).
func (r *Registry) Converter(from, to version.Version) (Converter, error) {
	if from == to {
		return nil, fmt.Errorf("%w: AIXM %v needs no conversion", ErrNoPath, from)
	}
	path, err := r.Path(from, to)
	if err != nil {
		return nil, err
	}
	return Chain(path...)
}

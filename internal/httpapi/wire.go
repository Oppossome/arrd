package httpapi

// Helpers for converting between the generated API types (apigen, the wire
// contract in tools/openapi.yaml) and the library's domain types. Each
// resource's converters live next to its handlers. Optional wire fields mirror
// the library's former `omitempty` JSON tags: zero values are left out.

// mapSlice converts every element; the result is never nil, so required
// arrays encode as [] rather than null.
func mapSlice[T, U any](in []T, f func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

// optSlice is mapSlice for optional arrays: empty becomes absent.
func optSlice[T, U any](in []T, f func(T) U) *[]U {
	if len(in) == 0 {
		return nil
	}
	out := mapSlice(in, f)
	return &out
}

func identity[T any](v T) T { return v }

// nonNil returns in, or an empty slice for nil.
func nonNil[T any](in []T) []T {
	if in == nil {
		return []T{}
	}
	return in
}

// optional returns a pointer to v, or nil for the zero value.
func optional[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// deref returns *p, or the zero value for nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// derefSlice returns *p, or nil for nil.
func derefSlice[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}

// nonNilMap returns m, or an empty map for nil, so required objects encode
// as {} rather than null.
func nonNilMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return map[K]V{}
	}
	return m
}

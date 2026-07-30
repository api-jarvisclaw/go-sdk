package jarvisclaw

// Pointer helpers for optional request fields.
//
// Optional scalars in request structs are pointers rather than plain values with
// `omitempty`, so that an explicit zero (0, 0.0, false) is still sent upstream
// instead of being silently dropped. These helpers make setting them inline
// readable:
//
//	Constraints: jarvisclaw.Constraints{MaxPriceUSD: jarvisclaw.Float64Ptr(0.01)}

// Float64Ptr returns a pointer to v.
func Float64Ptr(v float64) *float64 { return &v }

// IntPtr returns a pointer to v.
func IntPtr(v int) *int { return &v }

// BoolPtr returns a pointer to v.
func BoolPtr(v bool) *bool { return &v }

// StringPtr returns a pointer to v.
func StringPtr(v string) *string { return &v }

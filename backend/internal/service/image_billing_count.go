package service

// resolveBillableImageCount uses observed output whenever it is available.
// The opt-in fallback preserves a charge when a successful response omits its
// normal image array.
func resolveBillableImageCount(reported, requested int, fallbackToOne bool) int {
	if reported > 0 {
		return reported
	}
	if fallbackToOne {
		return 1
	}
	if requested > 0 {
		return requested
	}
	return 0
}

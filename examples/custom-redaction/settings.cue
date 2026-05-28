package main

#Config: {
	red_marker:  string @secret() @go(RedMarker,type=RedMarkerSecret)
	partial:     string @secret() @go(Partial,type=PartialSecret)
	fingerprint: string @secret() @go(Fingerprint,type=FingerprintSecret)
}

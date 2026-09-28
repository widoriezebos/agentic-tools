package main

// The helm (batch 4) ships its own repeat witnesses (HM-2); U-idem's
// registry names them.
func init() {
	registerIdempotency("helm take", idemStateful,
		"the same person already at the helm with the same reason: success (applied), the signature and helm.log untouched; a new reason keeps the since",
		witnessHelmTakeRepeat)
	registerIdempotency("helm return", idemStateful,
		"the machinery is already at the helm: success, nothing to return, no helm.log record",
		witnessHelmReturnRepeat)
}

//go:build batchtest

package main

var batchTestCapabilities = [...]batchCapability{
	recordStoreHistoryAndProberSeam, chainReaderIdentityUniquenessAndTransport,
	joinGate, assemblyConflictCeilingAndSeal, handedOverClaimCardinality, fieldCompleteHandover,
	serializedJoinPublication, crashSafeTerminalReturn, censusOnTheInjectedProber,
	boundedRolloutConfiguration, fifoLockAndStartRule, ownerVerbAndTick, productionSupervisorTakeover,
	registerPreservingFastForward, proofPlanningAndTipLaunch, revisionBoundAdmissionAndDiagnosticHeadroom,
	freshBaseDiagnosis, ejectionAndRedScheduling, trunkRedLedgerOwner, prefixReceipts,
	landingTransportHelpers, atomicSeriesAndRecovery, waitStatusDocsAndInventory,
}

func init() {
	for _, capability := range batchTestCapabilities {
		compiledBatchCapabilities[capability] = struct{}{}
	}
}

func unregisterBatchCapabilityForTest(capability batchCapability) func() {
	delete(compiledBatchCapabilities, capability)
	return func() { compiledBatchCapabilities[capability] = struct{}{} }
}

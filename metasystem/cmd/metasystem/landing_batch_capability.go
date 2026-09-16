package main

type batchCapability string

const (
	recordStoreHistoryAndProberSeam             batchCapability = "recordStoreHistoryAndProberSeam"
	chainReaderIdentityUniquenessAndTransport   batchCapability = "chainReaderIdentityUniquenessAndTransport"
	joinGate                                    batchCapability = "joinGate"
	assemblyConflictCeilingAndSeal              batchCapability = "assemblyConflictCeilingAndSeal"
	handedOverClaimCardinality                  batchCapability = "handedOverClaimCardinality"
	fieldCompleteHandover                       batchCapability = "fieldCompleteHandover"
	serializedJoinPublication                   batchCapability = "serializedJoinPublication"
	crashSafeTerminalReturn                     batchCapability = "crashSafeTerminalReturn"
	censusOnTheInjectedProber                   batchCapability = "censusOnTheInjectedProber"
	boundedRolloutConfiguration                 batchCapability = "boundedRolloutConfiguration"
	fifoLockAndStartRule                        batchCapability = "fifoLockAndStartRule"
	ownerVerbAndTick                            batchCapability = "ownerVerbAndTick"
	productionSupervisorTakeover                batchCapability = "productionSupervisorTakeover"
	registerPreservingFastForward               batchCapability = "registerPreservingFastForward"
	proofPlanningAndTipLaunch                   batchCapability = "proofPlanningAndTipLaunch"
	revisionBoundAdmissionAndDiagnosticHeadroom batchCapability = "revisionBoundAdmissionAndDiagnosticHeadroom"
	freshBaseDiagnosis                          batchCapability = "freshBaseDiagnosis"
	ejectionAndRedScheduling                    batchCapability = "ejectionAndRedScheduling"
	trunkRedLedgerOwner                         batchCapability = "trunkRedLedgerOwner"
	prefixReceipts                              batchCapability = "prefixReceipts"
	landingTransportHelpers                     batchCapability = "landingTransportHelpers"
	atomicSeriesAndRecovery                     batchCapability = "atomicSeriesAndRecovery"
	waitStatusDocsAndInventory                  batchCapability = "waitStatusDocsAndInventory"
)

var requiredBatchCapabilities = [...]batchCapability{
	recordStoreHistoryAndProberSeam, chainReaderIdentityUniquenessAndTransport,
	joinGate, assemblyConflictCeilingAndSeal, handedOverClaimCardinality,
	fieldCompleteHandover, serializedJoinPublication, crashSafeTerminalReturn,
	censusOnTheInjectedProber, boundedRolloutConfiguration, fifoLockAndStartRule,
	ownerVerbAndTick, productionSupervisorTakeover, registerPreservingFastForward,
	proofPlanningAndTipLaunch, revisionBoundAdmissionAndDiagnosticHeadroom,
	freshBaseDiagnosis, ejectionAndRedScheduling, trunkRedLedgerOwner,
	prefixReceipts, landingTransportHelpers, atomicSeriesAndRecovery,
	waitStatusDocsAndInventory,
}

var compiledBatchCapabilities = make(map[batchCapability]struct{})

func batchCapabilitiesAvailable() bool {
	for _, capability := range requiredBatchCapabilities {
		if _, ok := compiledBatchCapabilities[capability]; !ok {
			return false
		}
	}
	return true
}

package delegation

import (
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
)

// claimRequest is one claim-launch call's selection (its flags).
type claimRequest struct {
	opID, operationID, session, dispatchMode, resumedSession string
	runtime, model, role, aliasSource, reviews, launchMode   string
	permissionDigest, inputHash                              string
	productRoots                                             []string
	capMin                                                   string
	mainID, claimEpoch, goalID                               string
	goalRevision                                             uint64
	goalTier                                                 uint8
	gateWidth, machineID, approvedRef, destructiveReach      string
	adapterVerb                                              string
	creatorPID                                               int64
	occupancy                                                string
}

// claimLaunch is `job claim-launch` (and its --preflight): the typed claim
// state machine behind the delegate boundary's claim capability. It returns
// the printed result JSON and the verb's exit code.
func (s *session) claimLaunch(request claimRequest, preflight bool) (string, int) {
	operationID := request.operationID
	if operationID == "" {
		operationID = request.opID
	}
	binding := dispatch.DelegateClaimCapabilityBinding{
		JobID: request.opID, OperationID: operationID,
		DispatchMode: dispatch.DispatchMode(request.dispatchMode), AdapterVerb: request.adapterVerb,
	}
	if !s.claimAuthorized(binding, preflight) {
		return encodeJSON(dispatch.ClaimResult{
			Outcome: dispatch.ClaimRefusedInternalSurface,
			Evidence: map[string]any{
				"resolution": "delegate-verb-required",
				"remedy":     "use metasystem internal delegate",
			},
		}), 1
	}
	confPath := filepath.Join(s.root, "metasystem.conf")
	resolvedCap, _, _, err := dispatch.ResolveCap(confPath, request.role, request.runtime, config.CanonicalModel(request.model), request.aliasSource, request.capMin)
	if err != nil {
		s.eprintln(err.Error())
		return "", 1
	}
	resumed := request.resumedSession
	params := dispatch.ClaimLaunchParams{
		LookupEnv: s.configLookup(), Root: s.root, OpID: request.opID, OperationID: request.operationID,
		MainID: request.mainID, ClaimEpoch: request.claimEpoch, GoalID: request.goalID,
		GoalRevision: request.goalRevision, GoalTier: request.goalTier, MachineID: request.machineID,
		ApprovedRef: request.approvedRef, AdapterVerb: request.adapterVerb, GateWidth: request.gateWidth,
		Reviews: request.reviews,
		Request: dispatch.LaunchFingerprintRequest{
			SessionKey: request.session, DispatchMode: dispatch.DispatchMode(request.dispatchMode),
			ResumedSessionID: &resumed, Runtime: request.runtime, Model: request.model, Role: request.role,
			LaunchMode: dispatch.LaunchMode(request.launchMode), PermissionEnvelopeDigest: request.permissionDigest,
			ProductRoots: request.productRoots, CapMinutes: resolvedCap, InputHash: request.inputHash,
			GoalID: request.goalID, GoalRevision: request.goalRevision,
			DestructiveReach: dispatch.HazardClass(request.destructiveReach),
		},
		DefaultCapMinutes: resolvedCap,
	}
	if preflight {
		result, err := dispatch.ClaimLaunchPreflight(params)
		if err != nil {
			s.eprintln(err.Error())
			return "", 1
		}
		return encodeJSON(result), dispatch.ClaimOutcomeExitCode(result.Outcome)
	}
	if request.occupancy != "" {
		prepared, err := dispatch.ReadClaimOccupancyPreparation(request.occupancy, request.session)
		if err != nil {
			s.eprintln(err.Error())
			return "", 1
		}
		params.OccupancyPreparation = &prepared
	}
	processes, err := s.l.ports.Process.ClaimProcesses()
	if err != nil {
		s.eprintln(err.Error())
		return "", 1
	}
	root := s.root
	result, err := dispatch.ClaimLaunch(params, dispatch.ClaimLaunchDependencies{
		CreatorPID: request.creatorPID, IdentityReader: processes.Reader, ProcessVerifier: processes.Verifier,
		Reconcile: func(_, job string) (dispatch.ReconciliationResult, error) {
			return dispatch.ReconcileReservation(root, job, dispatch.ReconciliationDependencies{
				Scanner: processes.Scanner, Creator: processes.Reader,
				Emit: func(line string) { s.eprintln(line) },
			})
		},
	})
	if err != nil {
		s.eprintln(err.Error())
		return "", 1
	}
	return encodeJSON(result), dispatch.ClaimOutcomeExitCode(result.Outcome)
}

// claimAuthorized is the claim-launch surface authorization for this
// invocation's delegate standing.
func (s *session) claimAuthorized(binding dispatch.DelegateClaimCapabilityBinding, preflight bool) bool {
	return dispatchproc.ClaimAuthorized(s.root, dispatchproc.ClaimSurface{
		DelegateInternal: s.env.DelegateInternal, Capability: s.env.ClaimCapability,
	}, binding, preflight)
}

// reconcileReservation is `job reconcile-reservation` (its JSON is discarded
// by every lifecycle caller).
func (s *session) reconcileReservation(job string) error {
	processes, err := s.l.ports.Process.ClaimProcesses()
	if err != nil {
		return s.verbFailure(err)
	}
	_, err = dispatch.ReconcileReservation(s.root, job, dispatch.ReconciliationDependencies{
		Scanner: processes.Scanner, Creator: processes.Reader,
		Emit: func(line string) { s.eprintln(line) },
	})
	return s.verbFailure(err)
}

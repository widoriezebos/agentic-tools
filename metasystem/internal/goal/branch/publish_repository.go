package branch

type landPushRepository struct {
	RemoteTip func(repo, remote, ref string) (string, bool, error)
	Fetch     func(repo, remote, ref, destination string) error
	Ancestor  func(repo, older, newer string) error
	Clear     func(repo, ref string) error
	Publish   func(req LandPushRequest, prepared PreparedLanding) (CASOutcome, error)
}

func gitLandPushRepository() landPushRepository {
	transport := GitPushTransport{}
	return landPushRepository{
		RemoteTip: transport.RemoteTip,
		Fetch:     transport.Fetch,
		Ancestor: func(repo, older, newer string) error {
			_, err := gitOutput(repo, "merge-base", "--is-ancestor", older, newer)
			return err
		},
		Clear:   clearPushTxn,
		Publish: pushLandingAtomically,
	}
}

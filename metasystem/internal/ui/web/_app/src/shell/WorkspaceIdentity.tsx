import { Button, Chip, Skeleton } from "./controls";
import { useWorkspaceState } from "./identity";

/**
 * Who this workspace is, in the header.
 *
 * The self-hosted mark is ochre rather than red: a calm constant mark is read
 * more often than an alarm. The conflict — a self-hosted layout that also
 * records a template SHA — is the one identity case that is wrong, so it is
 * red, and its reason is on screen rather than in a tooltip.
 */
export function WorkspaceIdentity() {
  const { workspace, retry } = useWorkspaceState();

  if (workspace.state === "loading") {
    return (
      <div className="ms-identity">
        <Skeleton />
      </div>
    );
  }

  if (workspace.state === "failed") {
    return (
      <div className="ms-identity">
        <div className="ms-identity-lines">
          <span className="ms-identity-unknown">Workspace unknown</span>
          <span className="ms-identity-note">{workspace.message}</span>
        </div>
        <Button onClick={retry}>Retry</Button>
      </div>
    );
  }

  const described = workspace.workspace;
  if (described.conflict) {
    return (
      <div className="ms-identity">
        <div className="ms-identity-lines">
          <span className="ms-identity-subject ms-identity-subject--conflict">Workspace identity conflict</span>
          <span className="ms-identity-detail">
            self-hosted layout, adopted from <span className="ms-mono">{described.adoptedFrom.slice(0, 7)}</span>
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="ms-identity">
      <span className="ms-identity-subject">{described.subject}</span>
      {described.mode === "self-hosted" ? (
        <Chip marker>self-hosted</Chip>
      ) : (
        <span className="ms-identity-note">built with MetaSystem</span>
      )}
    </div>
  );
}

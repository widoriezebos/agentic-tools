import { Settings as SettingsIcon } from "lucide-react";
import type { ReactNode } from "react";

import { EmptyState, Pane } from "./Pane";
import { Button, Skeleton } from "../shell/controls";
import { useWorkspaceState } from "../shell/identity";
import type { Workspace } from "../shell/workspace";
import type { ThemePreference } from "../theme";
import { ThemeControl } from "../shell/ThemeControl";

/**
 * Settings, which is two live cards above one honest empty state: About this
 * workspace is what this build knows for certain, Appearance is the one
 * setting it can actually change, and the pages themselves are gate 7's.
 */
export function SettingsPane({
  theme,
  onTheme,
}: {
  theme: ThemePreference;
  onTheme: (preference: ThemePreference) => void;
}) {
  return (
    <Pane title="Settings">
      <div className="ms-pane-stack">
        <AboutCard />
        <section className="ms-card">
          <h2 className="ms-card-title">Appearance</h2>
          <ThemeControl preference={theme} onChange={onTheme} labelled />
        </section>
        <EmptyState id="settings" icon={SettingsIcon} />
      </div>
    </Pane>
  );
}

function AboutCard() {
  const { workspace, retry } = useWorkspaceState();
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">About this workspace</h2>
      {workspace.state === "loading" && <Skeleton />}
      {workspace.state === "failed" && (
        <div className="ms-composer-foot">
          <span className="ms-identity-unknown">
            Workspace unknown <span className="ms-identity-note">{workspace.message}</span>
          </span>
          <Button onClick={retry}>Retry</Button>
        </div>
      )}
      {workspace.state === "known" && <Facts workspace={workspace.workspace} />}
      <a className="ms-card-link" href="/THIRD-PARTY-NOTICES.txt">
        Open-source notices
      </a>
    </section>
  );
}

function Facts({ workspace }: { workspace: Workspace }) {
  return (
    <dl className="ms-facts">
      <Fact name="Subject">{workspace.subject}</Fact>
      <Fact name="Mode">{workspace.mode}</Fact>
      <Fact name="Checkout">
        <span className="ms-mono">{workspace.checkout}</span>
      </Fact>
      <Fact name="Installation">
        <span className="ms-mono">{workspace.installation}</span>
      </Fact>
      <Fact name="State root">
        <span className="ms-mono">{workspace.stateRoot}</span>
      </Fact>
      <Fact name="Engine build">
        <span className="ms-mono">{workspace.engineBuild}</span>
      </Fact>
      <Fact name="Started at">
        <span className="ms-mono">{workspace.startedAt}</span>
      </Fact>
      <Fact name="Source at HEAD">arrives with g1-s7</Fact>
      <Fact name="Adopted from">{adoption(workspace)}</Fact>
    </dl>
  );
}

/** What the installation records about where it came from, in its own words. */
function adoption(workspace: Workspace): ReactNode {
  if (workspace.adoptionRecord === "recorded") {
    return <span className="ms-mono">{workspace.adoptedFrom}</span>;
  }
  if (workspace.adoptionRecord === "placeholder" || workspace.adoptionRecord === "absent") {
    return "not recorded";
  }
  return "unreadable";
}

function Fact({ name, children }: { name: string; children: ReactNode }) {
  return (
    <>
      <dt className="ms-fact-name">{name}</dt>
      <dd className="ms-fact-value">{children}</dd>
    </>
  );
}

import { Settings as SettingsIcon } from "lucide-react";
import type { ReactNode } from "react";

import { EmptyState, Pane } from "./Pane";
import { dateAndTime, isLongHash, shortTip } from "../backlog/format";
import { Help } from "../help/Help";
import { Button, Skeleton } from "../shell/controls";
import { useWorkspaceState } from "../shell/identity";
import type { LandingGate, Store, Workspace } from "../shell/workspace";
import type { ThemePreference } from "../theme";
import { ThemeControl } from "../shell/ThemeControl";

/**
 * Settings, which is two live cards above one honest empty state: About this
 * workspace is what this build knows for certain, Appearance is the one
 * setting it can actually change, and the pages themselves are not built yet.
 *
 * A fact is said in words a person reads. What only a maintainer compares —
 * a whole build hash, a setting's key in the configuration file — stands
 * behind Details beside the fact it belongs to.
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
        <StoreCard />
        <LandingGateCard />
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
      {workspace.state === "known" && <AboutFacts workspace={workspace.workspace} />}
      <a className="ms-card-link" href="/THIRD-PARTY-NOTICES.txt">
        Open-source notices
      </a>
    </section>
  );
}

/**
 * The private store, which is the one thing on this page that is not about the
 * checkout: where the interface keeps this account's own material, what it holds
 * per workspace, and what it is kept to.
 *
 * It says nothing where the workspace resource is not there yet or could not be
 * read — the About card above carries that story for the same read, and a second
 * copy of it would be a second thing to keep true. A server that describes no
 * store, which is a seat whose account has no home the kit can read, says so in
 * the store's own words instead.
 */
function StoreCard() {
  const { workspace } = useWorkspaceState();
  if (workspace.state !== "known" || workspace.workspace.store === undefined) {
    return null;
  }
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">
        Private store <Help id="private-store" />
      </h2>
      <StoreFacts store={workspace.workspace.store} />
    </section>
  );
}

/**
 * The landing gate's two settings (g1-s70 D1), each with the source the layered
 * resolution reports: the environment, this seat's .local file, the committed
 * metasystem.conf, or the compiled default. It is the same resolution the
 * engine's gate and clock read, so what this card says is what binds them.
 */
function LandingGateCard() {
  const { workspace } = useWorkspaceState();
  if (workspace.state !== "known" || workspace.workspace.landingGate === undefined) {
    return null;
  }
  return (
    <section className="ms-card">
      <h2 className="ms-card-title">
        Landing gate <Help id="landing-gate" />
      </h2>
      <LandingGateFacts gate={workspace.workspace.landingGate} />
    </section>
  );
}

/** Where a setting's value came from, in words. */
export function sourceWords(source: string): string {
  switch (source) {
    case "env":
      return "from the environment";
    case "conf-local":
      return "from metasystem.conf.local";
    case "conf":
      return "from metasystem.conf";
    case "default":
      return "the default";
    default:
      return source;
  }
}

/** What each of the two settings decides, as the card names it. */
const GATE_NAMES: Readonly<Record<string, string>> = {
  "landing.review.human-from-tier": "Waits for a person from tier",
  "landing.review.auto-after": "Lands by itself after",
};

/** What the card calls a setting this build has no name for; its key is under Details. */
const UNNAMED_SETTING = "Another landing setting";

/** The two lines. Exported because the card reads the workspace from context. */
export function LandingGateFacts({ gate }: { gate: LandingGate }) {
  if (gate.problem !== undefined && gate.problem !== "") {
    return (
      <dl className="ms-facts">
        <Fact name="Settings">{gate.problem}</Fact>
      </dl>
    );
  }
  return (
    <dl className="ms-facts">
      {(gate.facts ?? []).map((fact) => (
        <Fact key={fact.key} name={GATE_NAMES[fact.key] ?? UNNAMED_SETTING}>
          <span className="ms-mono">{fact.value}</span> · {sourceWords(fact.source)}
          <Details>{fact.key}</Details>
        </Fact>
      ))}
    </dl>
  );
}

/**
 * The store's own lines. Exported because the card reads the workspace from
 * context and the lines are what a test reads.
 */
export function StoreFacts({ store }: { store: Store }) {
  if (store.problem !== undefined && store.problem !== "") {
    return (
      <dl className="ms-facts">
        <Fact name="Kept at">{store.problem}</Fact>
      </dl>
    );
  }
  return (
    <dl className="ms-facts">
      <Fact name="Kept at">
        <span className="ms-mono">{store.path}</span>
      </Fact>
      {(store.workspaces ?? []).map((one) => (
        <Fact key={one.name} name={one.current === true ? "This workspace" : "Another workspace"}>
          <span className="ms-mono">{one.name}</span> — {one.size}
        </Fact>
      ))}
      <Fact name="Kept to">{(store.bounds ?? []).join(" ")}</Fact>
    </dl>
  );
}

/**
 * What this build knows of the workspace. Exported because the card reads the
 * workspace from context and the lines are what a test reads.
 */
export function AboutFacts({ workspace }: { workspace: Workspace }) {
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
        <span className="ms-mono">{isLongHash(workspace.engineBuild) ? shortTip(workspace.engineBuild) : workspace.engineBuild}</span>
        {isLongHash(workspace.engineBuild) && <Details>{workspace.engineBuild}</Details>}
      </Fact>
      <Fact name="Started at">{dateAndTime(workspace.startedAt)}</Fact>
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

/** The value a maintainer compares and a reader does not, shut until it is asked for. */
function Details({ children }: { children: string }) {
  return (
    <details className="ms-fact-details">
      <summary>Details</summary>
      <span className="ms-mono">{children}</span>
    </details>
  );
}

function Fact({ name, children }: { name: string; children: ReactNode }) {
  return (
    <>
      <dt className="ms-fact-name">{name}</dt>
      <dd className="ms-fact-value">{children}</dd>
    </>
  );
}

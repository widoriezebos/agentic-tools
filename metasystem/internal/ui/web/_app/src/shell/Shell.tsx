import { useEffect, useMemo, useState } from "react";
import { Group, Panel, Separator, type Layout, type LayoutChangedMeta } from "react-resizable-panels";
import { Navigate, Route, Routes, useLocation } from "react-router";

import { AboutProvider } from "./about";
import { Drawer, type Caret } from "./Drawer";
import { ErrorBoundary } from "./ErrorBoundary";
import { Header } from "./Header";
import { useWorkspaceState, type WorkspaceState } from "./identity";
import { PHONE_QUERY, RAIL_QUERY, useMediaQuery } from "./media";
import { Rail } from "./Rail";
import { RefreshProvider } from "./refresh";
import { Sheet } from "./Sheet";
import { BacklogPane } from "../backlog/BacklogPane";
import { sectionHelp } from "../help/terms";
import { useNotifications } from "../notifications/store";
import { OverviewPane } from "../overview/OverviewPane";
import { PartnerProvider } from "../partner/store";
import { Focused } from "../panes/Focused";
import { NotFoundPane, SectionPane } from "../panes/sections";
import { SettingsPane } from "../panes/Settings";
import { DocumentPane } from "../project/DocumentPane";
import { GoalPane, ProjectPane } from "../project/ProjectPane";
import { activeSection, HOME_PATH, projectSections } from "../routes";
import {
  DEFAULT_DOCK_HEIGHT,
  MINIMUM_DOCK_HEIGHT,
  MINIMUM_WORK_HEIGHT,
  readDockHeight,
  readDockOpen,
  readRailExpanded,
  readTheme,
  writeDockHeight,
  writeDockOpen,
  writeRailExpanded,
  writeTheme,
} from "../storage";
import { applyTheme, effectiveTheme, systemIsDark, watchSystemTheme, type ThemePreference } from "../theme";
import { titleFor, type Identity } from "../title";

/**
 * The shell: a rail, a header, the work area, and the Project Partner drawer
 * along the bottom of it.
 *
 * The drawer is why the work area has one column again. As a panel on the
 * right it took four hundred pixels from the content at every width, and the
 * briefing's aside and the reader's outline went first; along the bottom it
 * takes forty-eight pixels and gives them back, and the same layout serves
 * every width. Two widths remain, and they are the rail's: expanded from
 * 1,128, collapsed below that, and on a phone behind the menu button.
 *
 * How much of the height an open drawer takes is the human's, through the
 * divider between the two, and is remembered. The work area is the group's
 * one panel while the drawer is closed, which is why the bar sits outside the
 * group: a divider to drag against a drawer that is only a bar would be a
 * handle for nothing.
 *
 * The focused view at /brain is the conversation in full, and shows no drawer:
 * the conversation is already in front of the human.
 */

/** The panels the divider sits between, named so their layout can be read. */
const WORK_PANEL = "work";
const DRAWER_PANEL = "partner";
export function Shell() {
  const location = useLocation();
  const phone = useMediaQuery(PHONE_QUERY);
  const railFits = useMediaQuery(RAIL_QUERY);

  const [railChoice, setRailChoice] = useState(() => readRailExpanded());
  const [drawerOpen, setDrawerOpen] = useState(() => readDockOpen());
  const [theme, setTheme] = useState<ThemePreference>(() => readTheme());
  const [systemDark, setSystemDark] = useState(() => systemIsDark());
  const [railSheetOpen, setRailSheetOpen] = useState(false);
  // Where the caret belongs once the drawer has opened or closed around it.
  // The draft itself belongs to the conversation store now: the bar's field,
  // the panel's and the focused page's are three elements for one sentence,
  // and the store is what all three read it from.
  const [caret, setCaret] = useState<Caret>("none");

  // The stored height is read once, as the layout this group opens with; from
  // there the group owns the arithmetic and a drag is what changes it.
  const [openedAt] = useState(() => readDockHeight());
  const layout = useMemo(() => ({ [WORK_PANEL]: 100 - openedAt, [DRAWER_PANEL]: openedAt }), [openedAt]);

  // The rail's stored choice applies only where an expanded rail fits; below
  // that the rail is collapsed and the choice is left untouched.
  const railExpanded = railFits && railChoice;

  const section = activeSection(location.pathname);
  const focused = section?.id === "brain";
  const sectionTitle = section?.title ?? "Not found";
  const { workspace } = useWorkspaceState();
  // What the steward has said and nobody has read. It leads the tab title,
  // because a human with six workspaces open reads the title bar first and a
  // count that is only in the header is a count they will not see.
  const { unread } = useNotifications();

  useEffect(() => watchSystemTheme(setSystemDark), []);

  useEffect(() => {
    applyTheme(document.documentElement, effectiveTheme(theme, systemDark));
  }, [theme, systemDark]);

  useEffect(() => {
    document.title = titleFor(sectionTitle, identityOf(workspace), unread);
  }, [sectionTitle, workspace, unread]);

  // The rail's sheet belongs to the phone. Crossing out of it closes the
  // sheet rather than leaving a modal layer over a layout that has no rail in
  // a sheet at all.
  useEffect(() => {
    if (!phone) {
      setRailSheetOpen(false);
    }
  }, [phone]);

  const chooseTheme = (preference: ThemePreference) => {
    setTheme(preference);
    writeTheme(preference);
  };

  const toggleRail = () => {
    const next = !railExpanded;
    setRailChoice(next);
    writeRailExpanded(next);
  };

  // Open or closed is remembered the way the dock's was, under the key the
  // dock used: it is the same preference about the same collaborator.
  const setDrawer = (next: boolean, goes: Caret) => {
    setDrawerOpen(next);
    setCaret(goes);
    writeDockOpen(next);
  };

  // From the header, whose button is not replaced and keeps the caret itself.
  const toggleDrawer = () => {
    setDrawer(!drawerOpen, "none");
  };

  // Only a human's own drag is remembered. A mount, a window resize, or a
  // layout the library recomputed carries isUserInteraction false and writes
  // nothing, so the stored height survives a trip through a short window.
  const remember = (next: Layout, meta: LayoutChangedMeta) => {
    const height = next[DRAWER_PANEL];
    if (meta.isUserInteraction && Number.isFinite(height)) {
      writeDockHeight(height);
    }
  };

  const panes = (
    <Routes>
      <Route path="/" element={<Navigate to={HOME_PATH} replace />} />
      <Route path="/brain" element={<Focused />} />
      {/* Backlog reads the ledger and Project reads the repository; the other
          four say which gate brings them. One goal is the ledger's, so its page
          is Backlog's, however much of the project's own records it shows.

          Both those pages are read a tab at a time, and the tab is the last
          segment of the address so that it can be sent to somebody and
          survives a reload. It is optional rather than a second route,
          because two routes would be two matches and a page that remounted —
          and refetched, and forgot what was open in it — every time a human
          moved between its own tabs. */}
      <Route path="/backlog" element={<BacklogPane />} />
      <Route path="/backlog/goal/:id/:tab?" element={<GoalPane />} />
      <Route path="/project/:tab?" element={<ProjectPane />} />
      <Route path="/project/doc/*" element={<DocumentPane />} />
      {/* Overview reads the project, the ledger and the journal at once, and
          is where "/" lands, so it is a route of its own rather than one of
          the sections that say which gate brings them. */}
      <Route path="/overview" element={<OverviewPane />} />
      {projectSections
        .filter(
          (candidate) =>
            candidate.id !== "backlog" && candidate.id !== "project" && candidate.id !== "overview",
        )
        .map((candidate) => (
          <Route key={candidate.id} path={candidate.path} element={<SectionPane section={candidate} />} />
        ))}
      <Route path="/settings" element={<SettingsPane theme={theme} onTheme={chooseTheme} />} />
      <Route path="*" element={<NotFoundPane />} />
    </Routes>
  );
  const work = <ErrorBoundary>{panes}</ErrorBoundary>;
  const drawn = drawerOpen && !focused;
  const drawer = (
    <ErrorBoundary>
      <Drawer
        open={drawn}
        about={sectionTitle}
        caret={caret}
        onCompose={() => {
          if (!drawn) {
            setDrawer(true, "panel");
          }
        }}
        onToggle={() => {
          setDrawer(!drawn, "toggle");
        }}
        onEscape={() => {
          setDrawer(false, "toggle");
        }}
      />
    </ErrorBoundary>
  );

  return (
    <AboutProvider>
      {/* The conversation stands above the drawer and the focused page, and
          below the page's own context: it reads what the pane says it is about
          so a question carries it. */}
      <PartnerProvider>
        {/* The section's refresh is offered by whichever pane is on screen and
            shown in the header, so the provider has to stand above both. */}
        <RefreshProvider>
        <div className="ms-shell">
          <a className="ms-skip-link" href="#content">
            Skip to content
          </a>
          {!phone && (
            <Rail
              expanded={railExpanded}
              toggleable={railFits}
              onToggle={toggleRail}
              theme={theme}
              onTheme={chooseTheme}
            />
          )}
          <div className="ms-shell-column">
            <Header
              sectionTitle={sectionTitle}
              help={sectionHelp(section?.id)}
              onMenu={
                phone
                  ? () => {
                      setRailSheetOpen(true);
                    }
                  : undefined
              }
              dockOpen={drawn}
              onDockToggle={toggleDrawer}
              focused={focused}
            />
            {focused ? (
              work
            ) : (
              <div className="ms-workarea">
                <Group
                  id="workarea"
                  className="ms-workarea-group"
                  orientation="vertical"
                  defaultLayout={layout}
                  onLayoutChanged={remember}
                >
                  <Panel id={WORK_PANEL} className="ms-work-panel" minSize={MINIMUM_WORK_HEIGHT}>
                    {work}
                  </Panel>
                  {drawn && (
                    <>
                      {/* The library's own double-click puts the panel back to
                          its default size; remembering that is this build's. */}
                      <Separator
                        className="ms-drawer-grip"
                        aria-label="Resize the Project Partner"
                        onDoubleClick={() => {
                          writeDockHeight(DEFAULT_DOCK_HEIGHT);
                        }}
                      />
                      <Panel
                        id={DRAWER_PANEL}
                        className="ms-drawer-panel-host"
                        minSize={MINIMUM_DOCK_HEIGHT}
                        defaultSize={`${String(DEFAULT_DOCK_HEIGHT)}%`}
                      >
                        {drawer}
                      </Panel>
                    </>
                  )}
                </Group>
                {!drawn && drawer}
              </div>
            )}
          </div>
          {phone && (
            <Sheet
              open={railSheetOpen}
              onOpenChange={setRailSheetOpen}
              side="left"
              label="Sections"
              title="Sections"
              closeLabel="Close the sections"
              bodyClassName="ms-sheet-body--rail"
            >
              <Rail
                inSheet
                expanded
                toggleable={false}
                onToggle={toggleRail}
                theme={theme}
                onTheme={chooseTheme}
                onNavigate={() => {
                  setRailSheetOpen(false);
                }}
              />
            </Sheet>
          )}
        </div>
        </RefreshProvider>
      </PartnerProvider>
    </AboutProvider>
  );
}

/** The tab title needs three cases, and the state carries four. */
function identityOf(workspace: WorkspaceState): Identity {
  if (workspace.state === "loading") {
    return { state: "loading" };
  }
  if (workspace.state === "failed") {
    return { state: "unknown" };
  }
  const described = workspace.workspace;
  return { state: "known", subject: described.subject, mode: described.mode, conflict: described.conflict };
}

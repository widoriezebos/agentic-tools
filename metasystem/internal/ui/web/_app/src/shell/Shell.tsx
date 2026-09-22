import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router";

import { AboutProvider } from "./about";
import { Drawer } from "./Drawer";
import { ErrorBoundary } from "./ErrorBoundary";
import { Header } from "./Header";
import { useWorkspaceState, type WorkspaceState } from "./identity";
import { PHONE_QUERY, RAIL_QUERY, useMediaQuery, WIDE_QUERY } from "./media";
import { Rail } from "./Rail";
import { Sheet } from "./Sheet";
import { BacklogPane } from "../backlog/BacklogPane";
import { Focused } from "../panes/Focused";
import { NotFoundPane, SectionPane } from "../panes/sections";
import { SettingsPane } from "../panes/Settings";
import { DocumentPane } from "../project/DocumentPane";
import { GoalPane, ProjectPane } from "../project/ProjectPane";
import { activeSection, HOME_PATH, projectSections } from "../routes";
import { readDockOpen, readRailExpanded, readTheme, writeDockOpen, writeRailExpanded, writeTheme } from "../storage";
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
 * The focused view at /brain is the conversation in full, and shows no drawer:
 * the conversation is already in front of the human.
 */
export function Shell() {
  const location = useLocation();
  const wide = useMediaQuery(WIDE_QUERY);
  const phone = useMediaQuery(PHONE_QUERY);
  const railFits = useMediaQuery(RAIL_QUERY);

  const [railChoice, setRailChoice] = useState(() => readRailExpanded());
  const [drawerOpen, setDrawerOpen] = useState(() => readDockOpen());
  const [theme, setTheme] = useState<ThemePreference>(() => readTheme());
  const [systemDark, setSystemDark] = useState(() => systemIsDark());
  const [railSheetOpen, setRailSheetOpen] = useState(false);

  // The rail's stored choice applies only where an expanded rail fits; below
  // that the rail is collapsed and the choice is left untouched.
  const railExpanded = railFits && railChoice;

  const section = activeSection(location.pathname);
  const focused = section?.id === "brain";
  const sectionTitle = section?.title ?? "Not found";
  const { workspace } = useWorkspaceState();

  useEffect(() => watchSystemTheme(setSystemDark), []);

  useEffect(() => {
    applyTheme(document.documentElement, effectiveTheme(theme, systemDark));
  }, [theme, systemDark]);

  useEffect(() => {
    document.title = titleFor(sectionTitle, identityOf(workspace));
  }, [sectionTitle, workspace]);

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
  const toggleDrawer = () => {
    const next = !drawerOpen;
    setDrawerOpen(next);
    writeDockOpen(next);
  };

  const panes = (
    <Routes>
      <Route path="/" element={<Navigate to={HOME_PATH} replace />} />
      <Route path="/brain" element={<Focused wide={wide} />} />
      {/* Backlog reads the ledger and Project reads the repository; the other
          four say which gate brings them. One goal is the ledger's, so its page
          is Backlog's, however much of the project's own records it shows. */}
      <Route path="/backlog" element={<BacklogPane />} />
      <Route path="/backlog/goal/:id" element={<GoalPane />} />
      <Route path="/project" element={<ProjectPane />} />
      <Route path="/project/doc/*" element={<DocumentPane />} />
      {projectSections
        .filter((candidate) => candidate.id !== "backlog" && candidate.id !== "project")
        .map((candidate) => (
          <Route key={candidate.id} path={candidate.path} element={<SectionPane section={candidate} />} />
        ))}
      <Route path="/settings" element={<SettingsPane theme={theme} onTheme={chooseTheme} />} />
      <Route path="*" element={<NotFoundPane />} />
    </Routes>
  );
  const work = <ErrorBoundary>{panes}</ErrorBoundary>;
  const drawn = drawerOpen && !focused;

  return (
    <AboutProvider>
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
        <div className="ms-column">
          <Header
            sectionTitle={sectionTitle}
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
            <div className="ms-workarea" data-open={drawn ? "true" : "false"}>
              {work}
              <ErrorBoundary>
                <Drawer open={drawn} about={sectionTitle} onToggle={toggleDrawer} />
              </ErrorBoundary>
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

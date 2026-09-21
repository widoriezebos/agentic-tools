import { Maximize2 } from "lucide-react";
import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router";

import { Dock, DockSheetBody } from "./Dock";
import { ErrorBoundary } from "./ErrorBoundary";
import { Header } from "./Header";
import { IconButton } from "./controls";
import { useWorkspaceState, type WorkspaceState } from "./identity";
import {
  PHONE_QUERY,
  RAIL_QUERY,
  RAIL_WIDTH_COLLAPSED,
  RAIL_WIDTH_EXPANDED,
  useMediaQuery,
  WIDE_QUERY,
} from "./media";
import { Panels } from "./Panels";
import { Rail } from "./Rail";
import { Sheet } from "./Sheet";
import { Focused } from "../panes/Focused";
import { NotFoundPane, SectionPane } from "../panes/sections";
import { SettingsPane } from "../panes/Settings";
import { activeSection, HOME_PATH, projectSections } from "../routes";
import { readDockOpen, readRailExpanded, readTheme, writeDockOpen, writeRailExpanded, writeTheme } from "../storage";
import { applyTheme, effectiveTheme, systemIsDark, watchSystemTheme, type ThemePreference } from "../theme";
import { titleFor, type Identity } from "../title";

/**
 * The shell: a rail, a header, the work area, and the Brain dock.
 *
 * Three widths. Wide, from 960, is the whole thing, with the rail expanded
 * only from 1,128 where 240 + 8 + 480 + 400 still fits. Compact keeps the
 * collapsed rail and turns the dock into a sheet. The phone hides the rail
 * behind the menu button and gives the dock the whole width.
 */
export function Shell() {
  const location = useLocation();
  const wide = useMediaQuery(WIDE_QUERY);
  const phone = useMediaQuery(PHONE_QUERY);
  const railFits = useMediaQuery(RAIL_QUERY);

  const [railChoice, setRailChoice] = useState(() => readRailExpanded());
  const [dockChoice, setDockChoice] = useState(() => readDockOpen());
  const [theme, setTheme] = useState<ThemePreference>(() => readTheme());
  const [systemDark, setSystemDark] = useState(() => systemIsDark());
  const [railSheetOpen, setRailSheetOpen] = useState(false);
  const [dockSheetOpen, setDockSheetOpen] = useState(false);

  // The rail's stored choice applies only where an expanded rail fits; below
  // that the rail is collapsed and the choice is left untouched.
  const railExpanded = railFits && railChoice;
  const railWidth = railExpanded ? RAIL_WIDTH_EXPANDED : RAIL_WIDTH_COLLAPSED;

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

  // A sheet belongs to one width. Crossing into another closes it rather than
  // leaving a modal layer over a layout that no longer has it.
  useEffect(() => {
    if (!phone) {
      setRailSheetOpen(false);
    }
    if (wide) {
      setDockSheetOpen(false);
    }
  }, [phone, wide]);

  const chooseTheme = (preference: ThemePreference) => {
    setTheme(preference);
    writeTheme(preference);
  };

  const toggleRail = () => {
    const next = !railExpanded;
    setRailChoice(next);
    writeRailExpanded(next);
  };

  const toggleDock = () => {
    if (wide) {
      const next = !dockChoice;
      setDockChoice(next);
      writeDockOpen(next);
      return;
    }
    setDockSheetOpen((open) => !open);
  };

  const closeDock = () => {
    setDockChoice(false);
    writeDockOpen(false);
  };

  const panes = (
    <Routes>
      <Route path="/" element={<Navigate to={HOME_PATH} replace />} />
      <Route path="/brain" element={<Focused wide={wide} />} />
      {projectSections.map((candidate) => (
        <Route key={candidate.id} path={candidate.path} element={<SectionPane section={candidate} />} />
      ))}
      <Route path="/settings" element={<SettingsPane theme={theme} onTheme={chooseTheme} />} />
      <Route path="*" element={<NotFoundPane />} />
    </Routes>
  );
  const work = <ErrorBoundary>{panes}</ErrorBoundary>;
  const docked = wide && dockChoice && !focused;

  return (
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
          dockOpen={wide ? dockChoice && !focused : dockSheetOpen}
          onDockToggle={toggleDock}
          focused={focused}
        />
        {docked ? (
          <Panels
            railWidth={railWidth}
            work={work}
            dock={
              <ErrorBoundary>
                <Dock onClose={closeDock} />
              </ErrorBoundary>
            }
          />
        ) : (
          work
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
      {!wide && !focused && (
        <Sheet
          open={dockSheetOpen}
          onOpenChange={setDockSheetOpen}
          id="brain-dock"
          side="right"
          label="Brain dock"
          title="Brain"
          closeLabel="Close the Brain dock"
          bodyClassName="ms-sheet-body--dock"
          actions={
            <ExpandButton
              onExpand={() => {
                setDockSheetOpen(false);
              }}
            />
          }
        >
          <DockSheetBody />
        </Sheet>
      )}
    </div>
  );
}

function ExpandButton({ onExpand }: { onExpand: () => void }) {
  const navigate = useNavigate();
  return (
    <IconButton
      label="Expand the Brain view"
      onClick={() => {
        onExpand();
        void navigate("/brain");
      }}
    >
      <Maximize2 size={16} strokeWidth={1.75} aria-hidden="true" />
    </IconButton>
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

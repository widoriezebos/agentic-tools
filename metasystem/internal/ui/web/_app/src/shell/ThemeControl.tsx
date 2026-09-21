import { Monitor, Moon, Sun } from "lucide-react";

import { Button, IconButton } from "./controls";
import { THEME_PREFERENCES, type ThemePreference } from "../theme";

/**
 * System, Light, Dark. The group is labelled, each button says which one it
 * is, and the chosen one is pressed rather than merely coloured.
 */

const icons = { system: Monitor, light: Sun, dark: Moon };
const labels = { system: "System", light: "Light", dark: "Dark" };

export function ThemeControl({
  preference,
  onChange,
  labelled = false,
}: {
  preference: ThemePreference;
  onChange: (preference: ThemePreference) => void;
  /** Settings shows the same control with its words; the rail shows icons. */
  labelled?: boolean;
}) {
  return (
    <div className="ms-switch ms-theme-control" role="group" aria-label="Theme">
      {THEME_PREFERENCES.map((candidate) => {
        const Icon = icons[candidate];
        const label = labels[candidate];
        const pressed = candidate === preference;
        if (labelled) {
          return (
            <Button
              key={candidate}
              aria-pressed={pressed}
              onClick={() => {
                onChange(candidate);
              }}
            >
              <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
              {label}
            </Button>
          );
        }
        return (
          <IconButton
            key={candidate}
            label={label}
            aria-pressed={pressed}
            onClick={() => {
              onChange(candidate);
            }}
          >
            <Icon size={16} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
        );
      })}
    </div>
  );
}

/*
 * The theme, before the first paint.
 *
 * This file is copied to the bundle's root and loaded from <head> ahead of the
 * stylesheet, so the page is painted once, in the right theme. It is a classic
 * script with a src, which the policy's script-src 'self' allows and an inline
 * script would not. Its key and attribute are src/theme.ts's, and
 * src/themeScript.test.ts holds the two files to the same names.
 */
(function () {
  var preference = "system";
  try {
    var stored = localStorage.getItem("ms.ui.theme");
    if (stored === "light" || stored === "dark" || stored === "system") {
      preference = stored;
    }
  } catch (error) {
    preference = "system";
  }
  var dark = preference === "dark";
  if (preference === "system") {
    dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  }
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
})();

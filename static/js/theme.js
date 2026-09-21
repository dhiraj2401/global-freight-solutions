// Runs synchronously in <head> to set the theme before first paint (no flash).
(function () {
  var stored = null;
  try {
    stored = localStorage.getItem("gfs-theme");
  } catch (e) {}
  var dark = stored ? stored === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
})();

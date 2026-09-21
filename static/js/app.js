// Progressive enhancements: theme toggle, mobile nav, track dialog, scroll
// reveal and htmx 4 form UX. The page is fully usable without this file.
(function () {
  "use strict";

  var root = document.documentElement;
  var THEME_KEY = "gfs-theme";

  // ---- Theme ---------------------------------------------------------------
  function storedTheme() {
    try {
      return localStorage.getItem(THEME_KEY);
    } catch (e) {
      return null;
    }
  }

  function applyTheme(theme, persist) {
    root.setAttribute("data-theme", theme);
    if (persist) {
      try {
        localStorage.setItem(THEME_KEY, theme);
      } catch (e) {}
    }
  }

  var media = window.matchMedia("(prefers-color-scheme: dark)");
  media.addEventListener("change", function (e) {
    if (!storedTheme()) applyTheme(e.matches ? "dark" : "light", false);
  });

  // ---- Mobile navigation ---------------------------------------------------
  var nav = document.getElementById("mobile-nav");
  var navToggle = document.querySelector("[data-nav-toggle]");

  function setNav(open) {
    if (!nav || !navToggle) return;
    nav.hidden = !open;
    navToggle.setAttribute("aria-expanded", String(open));
    navToggle.setAttribute("aria-label", open ? "Close menu" : "Open menu");
  }

  // ---- Delegated clicks ----------------------------------------------------
  document.addEventListener("click", function (e) {
    var el = e.target instanceof Element ? e.target : null;
    if (!el) return;

    if (el.closest("[data-theme-toggle]")) {
      applyTheme(root.getAttribute("data-theme") === "dark" ? "light" : "dark", true);
      return;
    }

    if (el.closest("[data-nav-toggle]")) {
      setNav(nav && nav.hidden);
      return;
    }

    if (el.closest("[data-nav-link]")) {
      setNav(false);
    }

    var opener = el.closest("[data-open-dialog]");
    if (opener) {
      var dialog = document.getElementById(opener.getAttribute("data-open-dialog"));
      if (dialog && typeof dialog.showModal === "function") {
        e.preventDefault();
        setNav(false);
        dialog.showModal();
        var input = dialog.querySelector("input");
        if (input) input.focus();
      }
      return;
    }

    // Close dialogs when clicking the backdrop.
    if (el.tagName === "DIALOG") el.close();
  });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && nav && !nav.hidden) {
      setNav(false);
      if (navToggle) navToggle.focus();
    }
  });

  // ---- Scroll reveal -------------------------------------------------------
  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if ("IntersectionObserver" in window && !reduceMotion) {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("is-visible");
            observer.unobserve(entry.target);
          }
        });
      },
      { rootMargin: "0px 0px -8% 0px", threshold: 0.08 }
    );
    document.querySelectorAll("[data-reveal]").forEach(function (el) {
      observer.observe(el);
    });
    root.classList.add("reveal-ready");
  }

  // ---- htmx 4 form UX ------------------------------------------------------
  function focusServerMessage(scope) {
    var target =
      scope.matches && scope.matches("[data-autofocus]")
        ? scope
        : scope.querySelector && scope.querySelector("[data-autofocus]");
    if (!target) return;
    target.removeAttribute("data-autofocus");
    target.focus({ preventScroll: true });
    var rect = target.getBoundingClientRect();
    if (rect.top < 64 || rect.bottom > window.innerHeight) {
      target.scrollIntoView({ block: "center", behavior: reduceMotion ? "auto" : "smooth" });
    }
  }

  if (window.htmx) {
    // onLoad runs for content htmx swaps in, so screen-reader and keyboard
    // users land on the success message or the error summary.
    htmx.onLoad(function (elt) {
      if (elt !== document.body) focusServerMessage(elt);
    });

    // htmx:error covers network failures and timeouts (htmx 4 consolidated
    // the old sendError/timeout events). Server errors arrive as HTML and are
    // swapped normally, which replaces the form; if the form is still on the
    // page afterwards, nothing came back and we tell the user.
    document.addEventListener("htmx:error", function (e) {
      var form = e.target instanceof Element ? e.target.closest("form") : null;
      if (!form) return;
      setTimeout(function () {
        if (!form.isConnected) return;
        var note = form.querySelector("[data-network-error]");
        if (!note) {
          note = document.createElement("p");
          note.setAttribute("data-network-error", "");
          note.setAttribute("role", "alert");
          note.className = "mt-3 text-center text-sm font-medium text-red-600";
          form.appendChild(note);
        }
        note.textContent = "We couldn't reach the server. Check your connection and try again.";
      }, 0);
    });
  }
})();

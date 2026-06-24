(function () {
  "use strict";
  var cfg = window.GFMVIEW || {};
  var app = document.getElementById("app");
  var contentEl = document.getElementById("content");
  var breadcrumbEl = document.getElementById("breadcrumb");
  var tocEl = document.getElementById("toc");
  var treeEl = document.getElementById("tree");
  var currentPath = cfg.initialPath || "";

  // ---- Theme ----
  var root = document.documentElement;
  var savedTheme = localStorage.getItem("gfmview-theme");
  if (savedTheme) root.setAttribute("data-theme", savedTheme);
  var toggle = document.getElementById("theme-toggle");
  if (toggle) {
    toggle.addEventListener("click", function () {
      var cur = root.getAttribute("data-theme");
      var order = ["auto", "light", "dark"];
      var next = order[(order.indexOf(cur) + 1) % order.length];
      root.setAttribute("data-theme", next);
      localStorage.setItem("gfmview-theme", next);
      toggle.title = "Theme: " + next;
    });
  }

  // ---- Sidebar width persistence + resizer ----
  var savedWidth = localStorage.getItem("gfmview-sidebar-width");
  if (savedWidth) root.style.setProperty("--gv-sidebar-width", savedWidth);
  var resizer = document.getElementById("resizer");
  if (resizer) {
    var dragging = false;
    resizer.addEventListener("mousedown", function (e) {
      e.preventDefault(); // suppress native text selection on drag start
      dragging = true;
      document.body.classList.add("resizing");
    });
    window.addEventListener("mouseup", function () {
      if (!dragging) return;
      dragging = false;
      document.body.classList.remove("resizing");
    });
    window.addEventListener("mousemove", function (e) {
      if (!dragging) return;
      e.preventDefault();
      var w = Math.min(Math.max(e.clientX, 150), 600) + "px";
      root.style.setProperty("--gv-sidebar-width", w);
      localStorage.setItem("gfmview-sidebar-width", w);
    });
  }

  // ---- Tree: collapse/expand + selection + filter ----
  function bindTree() {
    // Clicking a folder label (or its caret) toggles collapse.
    treeEl.querySelectorAll('.tree-item[data-dir="true"] > .tree-label').forEach(function (label) {
      label.addEventListener("click", function (e) {
        e.preventDefault();
        label.closest(".tree-item").classList.toggle("collapsed");
      });
    });
    treeEl.querySelectorAll("a.tree-label[data-path]").forEach(function (a) {
      a.addEventListener("click", function (e) {
        e.preventDefault();
        navigate(a.getAttribute("data-path"));
      });
    });
    markSelected();
  }

  function markSelected() {
    treeEl.querySelectorAll(".tree-item.selected").forEach(function (n) { n.classList.remove("selected"); });
    var a = treeEl.querySelector('a.tree-label[data-path="' + cssEscape(currentPath) + '"]');
    if (a) {
      var li = a.closest(".tree-item");
      li.classList.add("selected");
      // expand ancestors
      var p = li.parentElement;
      while (p && p !== treeEl) {
        if (p.classList && p.classList.contains("tree-item")) {
          p.classList.remove("collapsed");
        }
        p = p.parentElement;
      }
      scrollIntoViewVertical(a);
    }
  }

  // Scroll the tree vertically to reveal an element without ever changing the
  // horizontal scroll position (scrollIntoView would scroll sideways too).
  function scrollIntoViewVertical(el) {
    var container = treeEl;
    var cRect = container.getBoundingClientRect();
    var eRect = el.getBoundingClientRect();
    if (eRect.top < cRect.top) {
      container.scrollTop -= cRect.top - eRect.top;
    } else if (eRect.bottom > cRect.bottom) {
      container.scrollTop += eRect.bottom - cRect.bottom;
    }
  }

  var filter = document.getElementById("filter");
  if (filter) {
    filter.addEventListener("input", function () {
      var q = filter.value.trim().toLowerCase();
      treeEl.querySelectorAll(".tree-item").forEach(function (item) {
        if (item.getAttribute("data-dir") === "true") return;
        var name = (item.getAttribute("data-name") || "").toLowerCase();
        item.classList.toggle("hidden-by-filter", q !== "" && name.indexOf(q) === -1);
      });
      // hide dirs with no visible file descendants; expand dirs that match.
      treeEl.querySelectorAll('.tree-item[data-dir="true"]').forEach(function (dir) {
        var anyVisible = dir.querySelector('.tree-item[data-dir="false"]:not(.hidden-by-filter)');
        dir.classList.toggle("hidden-by-filter", q !== "" && !anyVisible);
        if (q !== "" && anyVisible) dir.classList.remove("collapsed");
      });
    });
  }

  function setAllCollapsed(collapsed) {
    treeEl.querySelectorAll('.tree-item[data-dir="true"]').forEach(function (dir) {
      dir.classList.toggle("collapsed", collapsed);
    });
  }
  var expandAllBtn = document.getElementById("expand-all");
  var collapseAllBtn = document.getElementById("collapse-all");
  if (expandAllBtn) expandAllBtn.addEventListener("click", function () { setAllCollapsed(false); });
  if (collapseAllBtn) collapseAllBtn.addEventListener("click", function () { setAllCollapsed(true); });

  function cssEscape(s) { return (window.CSS && CSS.escape) ? CSS.escape(s) : s.replace(/"/g, '\\"'); }

  // ---- Navigation ----
  function navigate(path, push, keepScroll) {
    fetch("/api/render?path=" + encodeURIComponent(path), { headers: { "Accept": "application/json" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      })
      .then(function (data) {
        currentPath = path;
        document.title = data.title || path || "gfm-hotview";
        var scrollContainer = contentEl.parentElement;
        var savedScroll = keepScroll ? scrollContainer.scrollTop : 0;
        contentEl.innerHTML = data.html;
        breadcrumbEl.innerHTML = data.breadcrumb || "";
        renderTOC(data.headings || []);
        enhanceContent();
        markSelected();
        scrollContainer.scrollTop = savedScroll;
        if (push !== false) {
          history.pushState({ path: path }, "", "/view/" + path.split("/").map(encodeURIComponent).join("/"));
        }
      })
      .catch(function (err) {
        contentEl.innerHTML = '<p class="error">Failed to load: ' + (err && err.message) + "</p>";
      });
  }

  window.addEventListener("popstate", function (e) {
    var p = (e.state && e.state.path) || decodeURIComponent(location.pathname.replace(/^\/view\//, ""));
    navigate(p, false);
  });

  // In-document relative links -> internal navigation
  contentEl.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest("a");
    if (!a) return;
    var href = a.getAttribute("href") || "";
    if (/^https?:\/\//i.test(href) || href.indexOf("//") === 0) { a.target = "_blank"; a.rel = "noopener"; return; }
    if (href.indexOf("#") === 0) return; // anchor handled by browser
    if (/\.(md|markdown)(#.*)?$/i.test(href)) {
      e.preventDefault();
      var base = currentPath.indexOf("/") >= 0 ? currentPath.replace(/\/[^/]*$/, "/") : "";
      var target = resolveRel(base, href.split("#")[0]);
      navigate(target);
    }
  });

  function resolveRel(base, rel) {
    if (rel.indexOf("/") === 0) return rel.replace(/^\/+/, "");
    var parts = (base + rel).split("/");
    var out = [];
    parts.forEach(function (p) {
      if (p === "." || p === "") return;
      if (p === "..") out.pop();
      else out.push(p);
    });
    return out.join("/");
  }

  // ---- TOC ----
  function renderTOC(headings) {
    if (!tocEl) return;
    var useful = headings.filter(function (h) { return h.level >= 2 && h.level <= 4 && h.id; });
    if (useful.length < 2) {
      app.classList.remove("has-toc");
      tocEl.innerHTML = "";
      return;
    }
    app.classList.add("has-toc");
    var html = '<div class="toc-title">On this page</div><ul>';
    useful.forEach(function (h) {
      html += '<li class="toc-l' + h.level + '"><a href="#' + h.id + '">' + escapeHTML(h.text) + "</a></li>";
    });
    html += "</ul>";
    tocEl.innerHTML = html;
    tocEl.querySelectorAll("a").forEach(function (a) {
      a.addEventListener("click", function (e) {
        e.preventDefault();
        var el = document.getElementById(a.getAttribute("href").slice(1));
        if (el) el.scrollIntoView({ behavior: "smooth" });
      });
    });
  }

  function escapeHTML(s) {
    var d = document.createElement("div"); d.textContent = s; return d.innerHTML;
  }

  // Scroll-spy
  var spyObserver = null;
  function setupScrollSpy() {
    if (spyObserver) spyObserver.disconnect();
    if (!tocEl || !app.classList.contains("has-toc")) return;
    var links = {};
    tocEl.querySelectorAll("a").forEach(function (a) { links[a.getAttribute("href").slice(1)] = a; });
    spyObserver = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) {
        if (en.isIntersecting) {
          Object.values(links).forEach(function (l) { l.classList.remove("active"); });
          var l = links[en.target.id];
          if (l) l.classList.add("active");
        }
      });
    }, { rootMargin: "0px 0px -70% 0px" });
    contentEl.querySelectorAll("h2[id], h3[id], h4[id]").forEach(function (h) { spyObserver.observe(h); });
  }

  // ---- Content enhancement: mermaid + math ----
  function enhanceContent() {
    if (window.mermaid) {
      try {
        var theme = root.getAttribute("data-theme");
        var dark = theme === "dark" || (theme === "auto" && window.matchMedia && matchMedia("(prefers-color-scheme: dark)").matches);
        window.mermaid.initialize({ startOnLoad: false, theme: dark ? "dark" : "default" });
        var blocks = contentEl.querySelectorAll("pre.mermaid, .mermaid");
        if (blocks.length) window.mermaid.run({ nodes: blocks });
      } catch (e) { /* ignore */ }
    }
    if (window.renderMathInElement) {
      try {
        window.renderMathInElement(contentEl, {
          delimiters: [
            { left: "$$", right: "$$", display: true },
            { left: "$", right: "$", display: false },
            { left: "\\(", right: "\\)", display: false },
            { left: "\\[", right: "\\]", display: true }
          ],
          throwOnError: false
        });
      } catch (e) { /* ignore */ }
    }
    setupScrollSpy();
  }

  // ---- Live reload (SSE) ----
  function connectSSE() {
    if (!cfg.reload || !window.EventSource) return;
    var status = document.getElementById("reload-status");
    var es = new EventSource("/events");
    es.addEventListener("content", function () { navigate(currentPath, false, true); });
    es.addEventListener("tree", function () { refreshTree(); });
    es.addEventListener("css", function () { reloadUserCSS(); });
    es.onopen = function () { if (status) status.hidden = true; };
    es.onerror = function () {
      if (status) status.hidden = false;
      es.close();
      setTimeout(connectSSE, 1500);
    };
  }

  function refreshTree() {
    fetch("/api/tree-html").then(function (r) { return r.text(); }).then(function (html) {
      treeEl.innerHTML = html;
      bindTree();
    }).catch(function () {});
  }

  function reloadUserCSS() {
    var link = document.querySelector('link[href^="/user.css"]');
    if (link) link.href = "/user.css?t=" + Date.now();
  }

  // ---- init ----
  bindTree();
  enhanceContent();
  connectSSE();
})();

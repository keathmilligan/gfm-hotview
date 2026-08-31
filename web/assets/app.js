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
    closeMediaZoom();
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

  // ---- Content enhancement: mermaid + math + copy buttons ----
  function enhanceContent() {
    if (window.mermaid) {
      try {
        var theme = root.getAttribute("data-theme");
        var dark = theme === "dark" || (theme === "auto" && window.matchMedia && matchMedia("(prefers-color-scheme: dark)").matches);
        window.mermaid.initialize({ startOnLoad: false, theme: dark ? "dark" : "default" });
        var blocks = contentEl.querySelectorAll("pre.mermaid, .mermaid");
        if (blocks.length) {
          var done = window.mermaid.run({ nodes: blocks });
          if (done && typeof done.then === "function") done.then(bindMermaidZoom, bindMermaidZoom);
          else bindMermaidZoom();
        }
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
    addCopyButtons();
    bindImageZoom();
    setupScrollSpy();
  }

  // ---- Copy-to-clipboard buttons on code blocks ----
  var COPY_ICON = '<svg class="copy-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>';
  var CHECK_ICON = '<svg class="copy-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>';
  var ZOOM_ICON = '<svg class="zoom-icon" viewBox="0 0 16 16" aria-hidden="true"><path d="M3.75 2A1.75 1.75 0 0 0 2 3.75v2.5a.75.75 0 0 0 1.5 0v-2.5a.25.25 0 0 1 .25-.25h2.5a.75.75 0 0 0 0-1.5ZM10 2a.75.75 0 0 0 0 1.5h2.5a.25.25 0 0 1 .25.25v2.5a.75.75 0 0 0 1.5 0v-2.5A1.75 1.75 0 0 0 12.5 2ZM2.75 10a.75.75 0 0 1 .75.75v2.5c0 .138.112.25.25.25h2.5a.75.75 0 0 1 0 1.5h-2.5A1.75 1.75 0 0 1 2 13.25v-2.5a.75.75 0 0 1 .75-.75Zm11 0a.75.75 0 0 1 .75.75v2.5A1.75 1.75 0 0 1 12.5 15h-2.5a.75.75 0 0 1 0-1.5h2.5a.25.25 0 0 0 .25-.25v-2.5a.75.75 0 0 1 .75-.75Z"/></svg>';

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text);
    }
    return new Promise(function (resolve, reject) {
      try {
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        var ok = document.execCommand("copy");
        document.body.removeChild(ta);
        if (ok) resolve(); else reject(new Error("copy failed"));
      } catch (e) { reject(e); }
    });
  }

  function addCopyButtons() {
    contentEl.querySelectorAll("pre").forEach(function (pre) {
      if (pre.classList.contains("mermaid")) return; // diagrams, not copyable text
      if (pre.parentElement && pre.parentElement.classList.contains("code-block")) return; // already wrapped
      var wrap = document.createElement("div");
      wrap.className = "code-block";
      pre.parentNode.insertBefore(wrap, pre);
      wrap.appendChild(pre);
      var btn = document.createElement("button");
      btn.className = "copy-btn";
      btn.type = "button";
      btn.title = "Copy code";
      btn.setAttribute("aria-label", "Copy code to clipboard");
      btn.innerHTML = COPY_ICON;
      btn.addEventListener("click", function () {
        var code = pre.querySelector("code");
        var text = (code ? code.textContent : pre.textContent) || "";
        copyText(text).then(function () {
          btn.classList.add("copied");
          btn.innerHTML = CHECK_ICON;
          btn.title = "Copied!";
          btn.setAttribute("aria-label", "Copied!");
          setTimeout(function () {
            btn.classList.remove("copied");
            btn.innerHTML = COPY_ICON;
            btn.title = "Copy code";
            btn.setAttribute("aria-label", "Copy code to clipboard");
          }, 1500);
        });
      });
      wrap.appendChild(btn);
    });
  }

  // ---- Media zoom overlay (Mermaid + images) ----
  var zoomEl = document.getElementById("media-zoom");
  var zoomViewport = document.getElementById("media-zoom-viewport");
  var zoomCanvas = document.getElementById("media-zoom-canvas");
  var zoomClose = document.getElementById("media-zoom-close");
  var zoomScale = 1;
  var zoomTx = 0;
  var zoomTy = 0;
  var zoomDragging = false;
  var zoomMoved = false;
  var zoomLastX = 0;
  var zoomLastY = 0;
  var zoomOrigin = null;

  function applyZoom() {
    if (!zoomCanvas) return;
    zoomCanvas.style.transform = "translate(" + zoomTx + "px, " + zoomTy + "px) scale(" + zoomScale + ")";
  }

  function showZoom(node, nw, nh) {
    if (!zoomEl || !zoomViewport || !zoomCanvas) return;
    zoomCanvas.innerHTML = "";
    zoomCanvas.appendChild(node);
    zoomEl.hidden = false;
    document.body.classList.add("media-zoom-open");
    zoomOrigin = document.activeElement;
    if (zoomClose) zoomClose.focus();
    var pad = 64;
    var vw = zoomViewport.clientWidth;
    var vh = zoomViewport.clientHeight;
    var fit = 1;
    if (nw > 0 && nh > 0 && vw > pad && vh > pad) {
      fit = Math.min((vw - pad) / nw, (vh - pad) / nh);
      if (!isFinite(fit) || fit <= 0) fit = 1;
    }
    zoomScale = fit;
    zoomTx = (vw - (nw || 0) * zoomScale) / 2;
    zoomTy = (vh - (nh || 0) * zoomScale) / 2;
    applyZoom();
  }

  function makeZoomButton(title) {
    var btn = document.createElement("button");
    btn.className = "media-zoom-btn";
    btn.type = "button";
    btn.title = title;
    btn.setAttribute("aria-label", title);
    btn.innerHTML = ZOOM_ICON;
    return btn;
  }

  function cloneMermaidSvg(svg) {
    var clone = svg.cloneNode(true);
    var prefix = "mz" + Date.now() + "-";
    var idMap = {};
    clone.querySelectorAll("[id]").forEach(function (n) {
      var old = n.getAttribute("id");
      if (!old) return;
      var neu = prefix + old;
      idMap[old] = neu;
      n.setAttribute("id", neu);
    });
    var rewriteVal = function (v) {
      if (!v) return v;
      var next = v.replace(/url\(#([^)]+)\)/g, function (m, id) {
        return idMap[id] ? "url(#" + idMap[id] + ")" : m;
      });
      if (next.charAt(0) === "#") {
        var hid = next.slice(1);
        if (idMap[hid]) next = "#" + idMap[hid];
      }
      return next;
    };
    clone.querySelectorAll("*").forEach(function (n) {
      for (var i = 0; i < n.attributes.length; i++) {
        var a = n.attributes[i];
        var rewritten = rewriteVal(a.value);
        if (rewritten !== a.value) n.setAttribute(a.name, rewritten);
      }
    });
    clone.querySelectorAll("style").forEach(function (st) {
      var t = st.textContent;
      Object.keys(idMap).sort(function (a, b) { return b.length - a.length; }).forEach(function (old) {
        t = t.split("#" + old).join("#" + idMap[old]);
      });
      st.textContent = t;
    });
    return clone;
  }

  function bindMermaidZoom() {
    contentEl.querySelectorAll("pre.mermaid, .mermaid").forEach(function (el) {
      if (el.closest(".mermaid-block")) return;
      if (!el.querySelector("svg")) return;
      var wrap = document.createElement("div");
      wrap.className = "mermaid-block";
      el.parentNode.insertBefore(wrap, el);
      wrap.appendChild(el);
      var btn = makeZoomButton("Zoom diagram");
      wrap.appendChild(btn);
      wrap.addEventListener("click", function (e) {
        if (e.target.closest("a")) return;
        openMermaidZoom(el);
      });
    });
  }

  function openMermaidZoom(el) {
    if (!zoomEl || !zoomViewport || !zoomCanvas) return;
    var svg = el.querySelector("svg");
    if (!svg) return;
    var clone = cloneMermaidSvg(svg);
    var vb = clone.viewBox && clone.viewBox.baseVal;
    var nw = (vb && vb.width) ? vb.width : 0;
    var nh = (vb && vb.height) ? vb.height : 0;
    if (!nw || !nh) {
      var wAttr = clone.getAttribute("width");
      var hAttr = clone.getAttribute("height");
      if (wAttr && !/%$/.test(wAttr)) nw = parseFloat(wAttr) || 0;
      if (hAttr && !/%$/.test(hAttr)) nh = parseFloat(hAttr) || 0;
    }
    if (!nw || !nh) {
      try {
        var bbox = svg.getBBox();
        nw = bbox.width;
        nh = bbox.height;
      } catch (err) { /* ignore */ }
    }
    if (nw && nh) {
      clone.setAttribute("width", nw);
      clone.setAttribute("height", nh);
      clone.style.width = nw + "px";
      clone.style.height = nh + "px";
    }
    clone.style.maxWidth = "none";
    showZoom(clone, nw, nh);
  }

  function bindImageZoom() {
    contentEl.querySelectorAll("img").forEach(function (img) {
      if (img.closest(".image-block, .mermaid-block, .mermaid")) return;
      var wrapTarget = img;
      if (img.parentElement && img.parentElement.tagName === "A") wrapTarget = img.parentElement;
      var wrap = document.createElement("span");
      wrap.className = "image-block";
      wrapTarget.parentNode.insertBefore(wrap, wrapTarget);
      wrap.appendChild(wrapTarget);
      var btn = makeZoomButton("Zoom image");
      btn.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        openImageZoom(img);
      });
      wrap.appendChild(btn);
      wrap.addEventListener("click", function (e) {
        if (e.target.closest("a")) return;
        if (e.target.closest(".media-zoom-btn")) return;
        openImageZoom(img);
      });
    });
  }

  function openImageZoom(img) {
    function go() {
      var nw = img.naturalWidth;
      var nh = img.naturalHeight;
      if (!nw || !nh) return;
      var clone = document.createElement("img");
      clone.src = img.currentSrc || img.src;
      clone.alt = img.alt || "";
      clone.style.maxWidth = "none";
      clone.style.width = nw + "px";
      clone.style.height = nh + "px";
      showZoom(clone, nw, nh);
    }
    if (img.complete && img.naturalWidth) go();
    else img.addEventListener("load", go, { once: true });
  }

  function closeMediaZoom() {
    if (!zoomEl || zoomEl.hidden) return;
    zoomEl.hidden = true;
    document.body.classList.remove("media-zoom-open");
    zoomDragging = false;
    if (zoomViewport) zoomViewport.classList.remove("panning");
    if (zoomCanvas) zoomCanvas.innerHTML = "";
    if (zoomOrigin && typeof zoomOrigin.focus === "function") {
      try { zoomOrigin.focus(); } catch (err) { /* ignore */ }
    }
    zoomOrigin = null;
  }

  if (zoomEl) {
    zoomEl.addEventListener("wheel", function (e) { e.preventDefault(); }, { passive: false });
  }
  if (zoomClose) {
    zoomClose.addEventListener("click", function (e) {
      e.stopPropagation();
      closeMediaZoom();
    });
  }

  if (zoomViewport) {
    zoomViewport.addEventListener("pointerdown", function (e) {
      if (e.button !== 0) return;
      if (e.target.closest("#media-zoom-close")) return;
      zoomDragging = true;
      zoomMoved = false;
      zoomLastX = e.clientX;
      zoomLastY = e.clientY;
      zoomViewport.classList.add("panning");
      if (zoomViewport.setPointerCapture) zoomViewport.setPointerCapture(e.pointerId);
      e.preventDefault();
    });
    zoomViewport.addEventListener("pointermove", function (e) {
      if (!zoomDragging) return;
      var dx = e.clientX - zoomLastX;
      var dy = e.clientY - zoomLastY;
      if (!zoomMoved && (dx * dx + dy * dy) > 16) zoomMoved = true;
      zoomLastX = e.clientX;
      zoomLastY = e.clientY;
      zoomTx += dx;
      zoomTy += dy;
      applyZoom();
    });
    function endPan(e) {
      if (!zoomDragging) return;
      zoomDragging = false;
      zoomViewport.classList.remove("panning");
      if (!zoomMoved && !e.target.closest("svg, img")) closeMediaZoom();
    }
    zoomViewport.addEventListener("pointerup", endPan);
    zoomViewport.addEventListener("pointercancel", endPan);
    zoomViewport.addEventListener("wheel", function (e) {
      e.preventDefault();
      var rect = zoomViewport.getBoundingClientRect();
      var mx = e.clientX - rect.left;
      var my = e.clientY - rect.top;
      var delta = e.deltaY;
      if (e.deltaMode === 1) delta *= 16;
      else if (e.deltaMode === 2) delta *= rect.height;
      var next = zoomScale * Math.exp(-delta * 0.002);
      if (next < 0.1) next = 0.1;
      if (next > 16) next = 16;
      var worldX = (mx - zoomTx) / zoomScale;
      var worldY = (my - zoomTy) / zoomScale;
      zoomScale = next;
      zoomTx = mx - worldX * zoomScale;
      zoomTy = my - worldY * zoomScale;
      applyZoom();
    }, { passive: false });
  }

  window.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closeMediaZoom();
  });

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

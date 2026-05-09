document.addEventListener("DOMContentLoaded", () => {
  initializeTheme();
  initializeSidebar();
  initializeFolderTree();
  initializeTOCScroll();
});

function initializeSidebar() {
  const isMobile = window.innerWidth <= 768;
  if (!isMobile) {
    const isCollapsed = localStorage.getItem("sidebar-collapsed") === "true";
    if (isCollapsed) {
      document.body.classList.add("sidebar-collapsed");
    }
  }
  updateSidebarIcon();
}

function initializeFolderTree() {
  const savedExpanded = getSavedFolderPaths();

  document.querySelectorAll(".tree-menu .folder").forEach((folder) => {
    const path = getFolderPath(folder);
    const shouldExpand = savedExpanded.includes(path) || folder.classList.contains("active-ancestor");
    folder.classList.toggle("expanded", shouldExpand);
  });

  // Scroll active file into view
  const activeLink = document.querySelector(".tree-menu .file a.active");
  if (activeLink) {
    activeLink.scrollIntoView({ block: "nearest" });
  }

  document.querySelectorAll(".tree-menu .folder-name").forEach((folderName) => {
    folderName.addEventListener("click", onFolderClick);
  });
}

function initializeTOCScroll() {
  const contentEl = document.querySelector(".content");
  if (!contentEl) return;

  document.querySelectorAll(".toc a").forEach((link) => {
    link.addEventListener("click", (e) => {
      e.preventDefault();
      const id = link.getAttribute("href").substring(1);
      const target = document.getElementById(id);
      if (!target) return;
      // offsetTop is relative to .content since headings are direct children of it
      contentEl.scrollTo({ top: target.offsetTop - 20, behavior: "smooth" });
    });
  });

  contentEl.addEventListener("scroll", updateTOCHighlight);
  updateTOCHighlight();
}

function updateTOCHighlight() {
  const contentEl = document.querySelector(".content");
  if (!contentEl) return;

  const sections = contentEl.querySelectorAll("h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]");
  const tocLinks = document.querySelectorAll(".toc a");
  const scrollTop = contentEl.scrollTop;

  let current = "";
  sections.forEach((section) => {
    if (scrollTop >= section.offsetTop - 60) {
      current = section.getAttribute("id");
    }
  });

  tocLinks.forEach((link) => {
    link.classList.toggle("active", link.getAttribute("href") === "#" + current);
  });
}

function onFolderClick(e) {
  e.stopPropagation();
  e.preventDefault();

  const folder = e.currentTarget.parentElement;
  const path = getFolderPath(folder);
  const isExpanded = folder.classList.contains("expanded");

  folder.classList.toggle("expanded", !isExpanded);

  let saved = getSavedFolderPaths();
  if (isExpanded) {
    saved = saved.filter((p) => p !== path);
  } else if (!saved.includes(path)) {
    saved.push(path);
  }
  localStorage.setItem("expandedFolders", JSON.stringify(saved));
}

function getFolderPath(folderEl) {
  return (
    folderEl.getAttribute("data-full-path") ||
    folderEl.querySelector(".folder-name")?.textContent ||
    ""
  );
}

function getSavedFolderPaths() {
  return JSON.parse(localStorage.getItem("expandedFolders") || "[]");
}

// Filter tree based on search input
function filterTree() {
  const query = document.getElementById("search-bar").value.toLowerCase().trim();
  const allItems = document.querySelectorAll(".tree-menu li");

  if (query === "") {
    allItems.forEach((item) => (item.style.display = ""));
    restoreFolderStates();
    return;
  }

  allItems.forEach((item) => (item.style.display = "none"));

  allItems.forEach((item) => {
    const nameEl = item.querySelector(".folder-name") || item.querySelector("a");
    const itemText = nameEl?.textContent.toLowerCase() || "";
    const itemPath = (item.dataset.fullPath || "").toLowerCase();

    if (!itemText.includes(query) && !itemPath.includes(query)) return;

    item.style.display = "block";

    if (item.classList.contains("folder")) {
      item.querySelectorAll("li").forEach((child) => (child.style.display = "block"));
    }

    showParents(item);
  });

  // Ensure folders with any visible children are themselves visible
  document.querySelectorAll(".tree-menu .folder").forEach((folder) => {
    const hasVisible = folder.querySelector('li:not([style*="display: none"])');
    if (hasVisible) {
      folder.style.display = "block";
      showParents(folder);
    }
  });
}

function showParents(el) {
  let parent = el.parentElement;
  while (parent && !parent.classList.contains("tree-menu")) {
    if (parent.tagName === "UL" || parent.tagName === "LI") {
      parent.style.display = "block";
    }
    parent = parent.parentElement;
  }
}

function restoreFolderStates() {
  const saved = getSavedFolderPaths();
  document.querySelectorAll(".tree-menu .folder").forEach((folder) => {
    folder.classList.toggle("expanded", saved.includes(getFolderPath(folder)));
  });
}

// Sidebar toggle
function toggleSidebar() {
  const isMobile = window.innerWidth <= 768;
  const body = document.body;

  if (isMobile) {
    const isOpen = body.classList.toggle("sidebar-mobile-open");
    isOpen ? createOverlay() : removeOverlay();
  } else {
    const isCollapsed = body.classList.toggle("sidebar-collapsed");
    localStorage.setItem("sidebar-collapsed", isCollapsed);
  }

  updateSidebarIcon();
}

function createOverlay() {
  if (document.querySelector(".sidebar-overlay")) return;
  const overlay = document.createElement("div");
  overlay.className = "sidebar-overlay";
  overlay.addEventListener("click", () => {
    document.body.classList.remove("sidebar-mobile-open");
    removeOverlay();
    updateSidebarIcon();
  });
  document.body.appendChild(overlay);
}

function removeOverlay() {
  document.querySelector(".sidebar-overlay")?.remove();
}

function updateSidebarIcon() {
  const icon = document.getElementById("sidebar-icon");
  if (!icon) return;
  const isMobile = window.innerWidth <= 768;
  if (isMobile) {
    icon.textContent = document.body.classList.contains("sidebar-mobile-open") ? "✕" : "☰";
  } else {
    icon.textContent = document.body.classList.contains("sidebar-collapsed") ? "→" : "☰";
  }
}

// Theme toggle
function toggleTheme() {
  const html = document.documentElement;
  const newTheme = html.getAttribute("data-theme") === "light" ? "dark" : "light";
  html.setAttribute("data-theme", newTheme);
  const icon = document.getElementById("theme-icon");
  if (icon) icon.textContent = newTheme === "light" ? "🌞" : "🌙";
  localStorage.setItem("theme", newTheme);
}

function initializeTheme() {
  const theme = localStorage.getItem("theme") || "light";
  document.documentElement.setAttribute("data-theme", theme);
  const icon = document.getElementById("theme-icon");
  if (icon) icon.textContent = theme === "light" ? "🌞" : "🌙";
}

window.addEventListener("resize", () => {
  const isMobile = window.innerWidth <= 768;
  const body = document.body;

  if (isMobile) {
    body.classList.remove("sidebar-collapsed");
    removeOverlay();
  } else {
    body.classList.remove("sidebar-mobile-open");
    removeOverlay();
    const isCollapsed = localStorage.getItem("sidebar-collapsed") === "true";
    body.classList.toggle("sidebar-collapsed", isCollapsed);
  }

  updateSidebarIcon();
});

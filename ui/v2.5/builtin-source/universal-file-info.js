(function () {
  "use strict";

  function moveImageFileInfo() {
    // already processed
    if (document.querySelector("[data-file-info-moved]")) return;

    // actual panel
    const fileInfo = document.querySelector(
      ".file-info-panel"
    );

    // details container
    const tabContent = document.querySelector(
      ".image-tabs .tab-content"
    );

    if (!fileInfo || !tabContent) return;

    // mark processed
    fileInfo.setAttribute("data-file-info-moved", "true");

    // remove bootstrap tab classes
    fileInfo.classList.remove(
      "tab-pane",
      "fade",
      "active",
      "show"
    );

    // remove tab-related attributes
    fileInfo.removeAttribute("role");
    fileInfo.removeAttribute("aria-labelledby");

    // remove ID so tabs can't target it
    fileInfo.removeAttribute("id");

    // remove File Info tab button
    const fileInfoTabButton = document.querySelector(
      'a[href*="file-info"], button[data-rb-event-key*="file-info"]'
    );

    if (fileInfoTabButton) {
      const navItem =
        fileInfoTabButton.closest("li") ||
        fileInfoTabButton.parentElement;

      if (navItem) {
        navItem.remove();
      } else {
        fileInfoTabButton.remove();
      }
    }

    // make panel visible normally
    fileInfo.style.display = "block";
    fileInfo.style.opacity = "1";

    // styling
    fileInfo.style.marginTop = "1rem";
    fileInfo.style.paddingTop = "1rem";
    fileInfo.style.borderTop =
      "1px solid rgba(255,255,255,0.08)";

    // move panel
    tabContent.prepend(fileInfo);
  }

  function init() {
    moveImageFileInfo();

    const observer = new MutationObserver(() => {
      moveImageFileInfo();
    });

    observer.observe(document.body, {
      childList: true,
      subtree: true,
    });
  }

  init();
})();
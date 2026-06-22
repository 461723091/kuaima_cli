(function () {
  let scheduled = false;

  function renderIcons() {
    scheduled = false;
    if (!window.lucide || typeof window.lucide.createIcons !== "function") {
      return;
    }
    window.lucide.createIcons({
      attrs: {
        "stroke-width": 2.2,
        "aria-hidden": "true",
      },
    });
  }

  function scheduleRender() {
    if (scheduled) {
      return;
    }
    scheduled = true;
    requestAnimationFrame(renderIcons);
  }

  window.KuaimaIcons = {
    render: scheduleRender,
  };

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", scheduleRender, { once: true });
  } else {
    scheduleRender();
  }

  window.addEventListener("load", scheduleRender);

  new MutationObserver((mutations) => {
    if (mutations.some((mutation) => mutation.addedNodes && mutation.addedNodes.length > 0)) {
      scheduleRender();
    }
  }).observe(document.documentElement, {
    childList: true,
    subtree: true,
  });
})();

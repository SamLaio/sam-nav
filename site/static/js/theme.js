(function () {
  const button = document.getElementById("theme-toggle");
  if (!button) return;

  const setButtonState = (theme) => {
    const nextTheme = theme === "dark" ? "light" : "dark";
    button.dataset.theme = theme;
    button.dataset.nextTheme = nextTheme;
    const nextLabel = nextTheme === "dark" ? button.dataset.darkLabel : button.dataset.lightLabel;
    button.title = nextLabel || button.title;
    button.setAttribute("aria-label", nextLabel || button.getAttribute("aria-label") || "");
  };

  const applyTheme = (theme) => {
    document.documentElement.dataset.theme = theme;
    setButtonState(theme);
    const themeSelect = document.querySelector('[name="defaultTheme"]');
    if (themeSelect) {
      themeSelect.value = theme;
    }
  };
  window.applySharedTheme = applyTheme;

  setButtonState(document.documentElement.dataset.theme || button.dataset.theme || "light");

  button.addEventListener("click", async () => {
    const currentTheme = document.documentElement.dataset.theme || "light";
    const nextTheme = currentTheme === "dark" ? "light" : "dark";
    try {
      const response = await fetch("/api/theme", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ theme: nextTheme })
      });
      const payload = await response.json();
      if (!response.ok || !payload.success) {
        throw new Error(payload.errorMessage || "Theme update failed");
      }
      applyTheme(payload.data.defaultTheme || nextTheme);
    } catch (error) {}
  });
})();

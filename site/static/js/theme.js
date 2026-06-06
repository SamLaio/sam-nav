(function () {
  const button = document.getElementById("theme-toggle");
  if (!button) return;
  const storageKey = "samNav.front.theme";
  const shouldRememberTheme = button.dataset.rememberTheme === "true";

  const readStoredTheme = () => {
    if (!shouldRememberTheme) return "";
    try {
      const theme = window.localStorage.getItem(storageKey);
      return theme === "dark" || theme === "light" ? theme : "";
    } catch (_) {
      return "";
    }
  };

  const writeStoredTheme = (theme) => {
    if (!shouldRememberTheme || (theme !== "dark" && theme !== "light")) return;
    try {
      window.localStorage.setItem(storageKey, theme);
    } catch (_) {}
  };

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

  applyTheme(readStoredTheme() || document.documentElement.dataset.theme || button.dataset.theme || "light");

  button.addEventListener("click", async () => {
    const currentTheme = document.documentElement.dataset.theme || "light";
    const nextTheme = currentTheme === "dark" ? "light" : "dark";
    applyTheme(nextTheme);
    writeStoredTheme(nextTheme);
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
      const savedTheme = payload.data.defaultTheme || nextTheme;
      applyTheme(savedTheme);
      writeStoredTheme(savedTheme);
    } catch (error) {}
  });
})();

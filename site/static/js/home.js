(function () {
  const input = document.getElementById("site-search");
  if (!input) return;

  document.addEventListener("keydown", (event) => {
    const target = event.target;
    const isTyping = target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);
    if (!isTyping && event.key.length === 1) {
      input.focus();
    }
  });
})();

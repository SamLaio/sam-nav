(function () {
  const input = document.getElementById("site-search");
  if (!input) return;
  const cards = Array.from(document.querySelectorAll(".link-card"));
  const categoryButtons = Array.from(document.querySelectorAll(".category-nav-button"));
  const empty = document.getElementById("search-empty");
  const searchEnginePanel = document.getElementById("search-engine-panel");
  const searchEngineLinks = Array.from(document.querySelectorAll(".search-engine-link"));
  const categoryStorageKey = "samNav.front.category";
  let selectedCategory = "";

  const readStoredCategory = () => {
    try {
      return window.localStorage.getItem(categoryStorageKey) || "";
    } catch (_) {
      return "";
    }
  };

  const writeStoredCategory = (category) => {
    try {
      window.localStorage.setItem(categoryStorageKey, category || "");
    } catch (_) {}
  };

  const normalizeText = (value) => String(value || "").trim().toLocaleLowerCase();

  const fallbackIconURL = (rawURL) => {
    try {
      const url = new URL(rawURL);
      return `https://www.google.com/s2/favicons?domain=${encodeURIComponent(url.hostname)}&sz=64`;
    } catch (_) {
      return "";
    }
  };

  const showFallbackLetter = (icon) => {
    icon.textContent = icon.dataset.fallbackLetter || "";
    icon.classList.add("is-letter-icon");
  };

  document.querySelectorAll(".link-icon img").forEach((image) => {
    const handleImageError = () => {
      const icon = image.closest(".link-icon");
      const card = image.closest(".link-card");
      if (!icon || !card) return;
      const fallbackURL = fallbackIconURL(card.href);
      if (fallbackURL && image.src !== fallbackURL) {
        image.src = fallbackURL;
        return;
      }
      image.remove();
      showFallbackLetter(icon);
    };
    image.addEventListener("error", handleImageError);
    if (image.complete && image.naturalWidth === 0) {
      handleImageError();
    }
  });

  const setCardVisible = (card, visible) => {
    card.hidden = !visible;
    card.classList.toggle("is-filter-hidden", !visible);
  };

  const applyFilters = () => {
    const keyword = normalizeText(input.value);
    const rawKeyword = input.value.trim();
    let visibleCount = 0;

    cards.forEach((card) => {
      const content = normalizeText(card.dataset.search);
      const cardCategory = card.dataset.category || "";
      const matchesKeyword = !keyword || content.includes(keyword);
      const matchesCategory = !selectedCategory || cardCategory === selectedCategory;
      const visible = matchesKeyword && matchesCategory;
      setCardVisible(card, visible);
      if (visible) visibleCount += 1;
    });

    if (empty) {
      empty.hidden = visibleCount > 0;
    }
    searchEngineLinks.forEach((link) => {
      const template = link.getAttribute("data-search-template") || "";
      link.href = template.includes("%s")
        ? template.replace("%s", encodeURIComponent(rawKeyword))
        : "#";
    });
    if (searchEnginePanel) {
      searchEnginePanel.hidden = !rawKeyword || searchEngineLinks.length === 0;
    }
  };

  document.addEventListener("keydown", (event) => {
    const target = event.target;
    const isTyping = target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);
    if (!isTyping && event.key.length === 1) {
      input.focus();
    }
  });

  categoryButtons.forEach((button) => {
    button.addEventListener("click", () => {
      selectedCategory = button.dataset.categoryFilter || "";
      writeStoredCategory(selectedCategory);
      categoryButtons.forEach((item) => {
        item.classList.toggle("is-active", item === button);
      });
      applyFilters();
    });
  });

  const storedCategory = readStoredCategory();
  const storedCategoryButton = categoryButtons.find((button) => (button.dataset.categoryFilter || "") === storedCategory);
  if (storedCategoryButton) {
    selectedCategory = storedCategory;
    categoryButtons.forEach((item) => {
      item.classList.toggle("is-active", item === storedCategoryButton);
    });
  } else {
    writeStoredCategory("");
  }

  input.addEventListener("input", applyFilters);

  input.addEventListener("keydown", (event) => {
    const keyword = input.value.trim();
    const firstEngine = searchEngineLinks.find((link) => (link.getAttribute("data-search-template") || "").includes("%s"));
    const searchEngineURL = firstEngine ? firstEngine.getAttribute("data-search-template") : "";
    if (event.key !== "Enter" || !keyword || !searchEngineURL.includes("%s")) return;
    event.preventDefault();
    window.location.href = searchEngineURL.replace("%s", encodeURIComponent(keyword));
  });

  applyFilters();
})();

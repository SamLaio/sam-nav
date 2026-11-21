(function () {
  const input = document.getElementById("site-search");
  if (!input) return;
  const cards = Array.from(document.querySelectorAll(".link-card"));
  const categoryButtons = Array.from(document.querySelectorAll(".category-nav-button"));
  const empty = document.getElementById("search-empty");
  const searchEnginePanel = document.getElementById("search-engine-panel");
  const searchEngineLinks = Array.from(document.querySelectorAll(".search-engine-link"));
  const searchHistoryPanel = document.getElementById("search-history-panel");
  const searchHistorySuggestions = document.getElementById("search-history-suggestions");
  const categoryStorageKey = "samNav.front.category";
  let selectedCategory = "";
  let searchHistoryTimer = 0;

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

  const requestJSON = async (url, options) => {
    const response = await fetch(url, {
      headers: {
        "Content-Type": "application/json"
      },
      ...options
    });
    const payload = await response.json();
    if (!response.ok || !payload.success) {
      throw new Error(payload.errorMessage || "Operation failed");
    }
    return payload.data;
  };

  const recordSearchHistory = (keyword) => {
    keyword = String(keyword || "").trim();
    if (input.dataset.authenticated !== "true" || !keyword) return;
    fetch("/api/search-history", {
      method: "POST",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify({ keyword }),
      keepalive: true
    }).catch(() => {});
  };

  const hideSearchHistory = () => {
    if (searchHistoryPanel) searchHistoryPanel.hidden = true;
  };

  const renderSearchHistory = (items) => {
    if (!searchHistoryPanel || !searchHistorySuggestions) return;
    searchHistorySuggestions.innerHTML = "";
    if (!items.length || document.activeElement !== input) {
      hideSearchHistory();
      return;
    }
    items.forEach((item) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "search-history-suggestion";
      button.textContent = item.keyword || "";
      button.addEventListener("mousedown", (event) => event.preventDefault());
      button.addEventListener("click", () => {
        input.value = item.keyword || "";
        applyFilters();
        hideSearchHistory();
        input.focus();
      });
      searchHistorySuggestions.appendChild(button);
    });
    searchHistoryPanel.hidden = false;
  };

  const refreshSearchHistory = async () => {
    if (input.dataset.authenticated !== "true" || !searchHistoryPanel || !searchHistorySuggestions || document.activeElement !== input) return;
    const keyword = input.value.trim();
    if (!keyword) {
      hideSearchHistory();
      return;
    }
    try {
      const items = await requestJSON(`/api/search-history?q=${encodeURIComponent(keyword)}&limit=10`);
      renderSearchHistory(items || []);
    } catch (_) {
      hideSearchHistory();
    }
  };

  const queueSearchHistoryRefresh = () => {
    window.clearTimeout(searchHistoryTimer);
    searchHistoryTimer = window.setTimeout(refreshSearchHistory, 120);
  };

  const directURLFromKeyword = (value) => {
    const raw = String(value || "").trim();
    if (!raw || /\s/.test(raw)) return "";

    const hasWebScheme = /^https?:\/\//i.test(raw);
    const hasProtocolRelativeURL = raw.startsWith("//");
    const candidate = hasWebScheme
      ? raw
      : hasProtocolRelativeURL
        ? `${window.location.protocol}${raw}`
        : `https://${raw}`;

    try {
      const url = new URL(candidate);
      const host = url.hostname;
      const isLikelyHost = host === "localhost" || host.includes(".") || /^\d{1,3}(\.\d{1,3}){3}$/.test(host) || Boolean(url.port);
      if ((hasWebScheme || hasProtocolRelativeURL || isLikelyHost) && ["http:", "https:"].includes(url.protocol)) {
        return url.href;
      }
    } catch (_) {}
    return "";
  };

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
  input.addEventListener("input", queueSearchHistoryRefresh);

  document.addEventListener("click", (event) => {
    if (event.target === input || event.target.closest("#search-history-panel")) return;
    hideSearchHistory();
  });

  input.addEventListener("keydown", (event) => {
    const keyword = input.value.trim();
    if (event.key === "Escape") {
      hideSearchHistory();
      return;
    }
    if (event.key !== "Enter" || !keyword) return;

    event.preventDefault();
    recordSearchHistory(keyword);
    const directURL = directURLFromKeyword(keyword);
    if (directURL) {
      window.location.href = directURL;
      return;
    }

    const firstEngine = searchEngineLinks.find((link) => (link.getAttribute("data-search-template") || "").includes("%s"));
    const searchEngineURL = firstEngine ? firstEngine.getAttribute("data-search-template") : "";
    if (!searchEngineURL.includes("%s")) return;
    window.location.href = searchEngineURL.replace("%s", encodeURIComponent(keyword));
  });

  searchEngineLinks.forEach((link) => {
    link.addEventListener("click", () => {
      recordSearchHistory(input.value.trim());
    });
  });

  applyFilters();
})();

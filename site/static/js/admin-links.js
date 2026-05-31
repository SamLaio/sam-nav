(function () {
  const list = document.getElementById("sortable-links");
  const createForm = document.getElementById("create-link-form");
  const editForm = document.getElementById("edit-link-form");
  const searchEngineForm = document.getElementById("search-engine-form");
  const editSearchEngineForm = document.getElementById("edit-search-engine-form");
  const createDialog = document.getElementById("create-link-dialog");
  const editDialog = document.getElementById("edit-link-dialog");
  const editSearchEngineDialog = document.getElementById("edit-search-engine-dialog");
  const categoryForm = document.getElementById("category-form");
  const settingsForm = document.getElementById("settings-form");
  const adminSettingsForm = document.getElementById("admin-settings-form");
  const message = document.getElementById("admin-message");
  const categoryMessage = document.getElementById("category-message");
  const settingsMessage = document.getElementById("settings-message");
  const adminSettingsMessage = document.getElementById("admin-settings-message");
  const importExportMessage = document.getElementById("import-export-message");
  const operationOverlay = document.getElementById("operation-overlay");
  const operationOverlayMessage = document.getElementById("operation-overlay-message");
  const operationCompleteDialog = document.getElementById("operation-complete-dialog");
  const operationCompleteMessage = document.getElementById("operation-complete-message");
  const empty = document.getElementById("admin-empty");
  const categoryFilter = document.getElementById("category-filter");
  const boxList = document.getElementById("box-list");
  const boxAsideList = document.getElementById("box-aside-list");
  const boxEmpty = document.getElementById("box-empty");
  const boxCount = document.getElementById("box-count");
  const searchEngineList = document.getElementById("search-engine-list");
  const searchEngineEmpty = document.getElementById("search-engine-empty");
  const searchEngineCount = document.getElementById("search-engine-count");
  const searchEngineMessage = document.getElementById("search-engine-message");
  const adminLayout = document.querySelector(".admin-layout");
  if (!list || !createForm || !editForm) return;

  let links = [];
  let categories = [];
  let searchEngines = [];
  let activeCategoryCombobox = null;
  let completeDialogResolver = null;
  const t = (key, fallback) => list.dataset[key] || fallback;

  const showMessage = (target, text, isError) => {
    if (!target) return;
    target.textContent = text || "";
    target.classList.toggle("is-error", Boolean(isError));
  };

  const requestJSON = async (url, options) => {
    const response = await fetch(url, {
      headers: {
        "Content-Type": "application/json"
      },
      ...options
    });
    const payload = await response.json();
    if (response.status === 401) {
      window.location.href = "/admin/login";
      throw new Error(payload.errorMessage || t("errorFallback", "Unauthorized"));
    }
    if (!response.ok || !payload.success) {
      throw new Error(payload.errorMessage || t("errorFallback", "Operation failed"));
    }
    return payload.data;
  };

  const requestDownload = async (url) => {
    const response = await fetch(url);
    if (response.status === 401) {
      window.location.href = "/admin/login";
      throw new Error(t("errorFallback", "Unauthorized"));
    }
    if (!response.ok) {
      const payload = await response.json();
      throw new Error(payload.errorMessage || t("errorFallback", "Operation failed"));
    }
    return response;
  };

  const filenameFromResponse = (response, fallback) => {
    const disposition = response.headers.get("Content-Disposition") || "";
    const match = disposition.match(/filename="([^"]+)"/);
    return match ? match[1] : fallback;
  };

  const downloadBlob = (blob, filename) => {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  };

  const switchView = (name) => {
    document.querySelectorAll("[data-view-target]").forEach((button) => {
      button.classList.toggle("is-active", button.dataset.viewTarget === name);
    });
    document.querySelectorAll("[data-view]").forEach((view) => {
      view.classList.toggle("is-active", view.dataset.view === name);
    });
    let activeAsideCount = 0;
    document.querySelectorAll("[data-aside-view]").forEach((view) => {
      const active = view.dataset.asideView === name;
      view.classList.toggle("is-active", active);
      if (active) activeAsideCount += 1;
    });
    adminLayout?.classList.toggle("is-aside-hidden", activeAsideCount === 0);
  };

  const openDialog = (dialog) => {
    if (!dialog) return;
    if (dialog.showModal) {
      dialog.showModal();
      return;
    }
    dialog.setAttribute("open", "");
  };

  const closeDialog = (dialog) => {
    if (!dialog) return;
    if (dialog.close) {
      dialog.close();
      return;
    }
    dialog.removeAttribute("open");
  };

  const showOperationOverlay = (text) => {
    if (!operationOverlay) return;
    if (operationOverlayMessage) {
      operationOverlayMessage.textContent = text || t("messageOperationProcessing", "Processing...");
    }
    operationOverlay.hidden = false;
  };

  const hideOperationOverlay = () => {
    if (!operationOverlay) return;
    operationOverlay.hidden = true;
  };

  const resolveCompleteDialog = () => {
    if (!completeDialogResolver) return;
    const resolve = completeDialogResolver;
    completeDialogResolver = null;
    resolve();
  };

  const showCompleteDialog = (text) => {
    if (!operationCompleteDialog) {
      window.alert(text);
      return Promise.resolve();
    }
    if (operationCompleteMessage) {
      operationCompleteMessage.textContent = text;
    }
    return new Promise((resolve) => {
      completeDialogResolver = resolve;
      openDialog(operationCompleteDialog);
    });
  };

  document.querySelectorAll("[data-view-target]").forEach((button) => {
    button.addEventListener("click", () => switchView(button.dataset.viewTarget));
  });

  operationCompleteDialog?.addEventListener("close", resolveCompleteDialog);

  document.querySelectorAll('[data-action="close-complete-dialog"]').forEach((button) => {
    button.addEventListener("click", () => {
      closeDialog(operationCompleteDialog);
      resolveCompleteDialog();
    });
  });

  const loadLinks = async () => {
    const [linkData, categoryData, searchEngineData] = await Promise.all([
      requestJSON("/api/admin/links"),
      requestJSON("/api/admin/categories"),
      requestJSON("/api/admin/search-engines")
    ]);
    links = linkData;
    categories = categoryData;
    searchEngines = searchEngineData;
    renderCategoryFilter();
    renderCategoryComboboxes();
    renderLinks();
    renderBoxes();
    renderSearchEngines();
  };

  const renderLinks = () => {
    list.innerHTML = "";
    const selectedCategory = categoryFilter ? categoryFilter.value : "";
    const visibleLinks = selectedCategory
      ? links.filter((link) => link.category === selectedCategory)
      : links;

    visibleLinks.forEach((link) => {
      const item = document.createElement("li");
      item.dataset.id = String(link.id);
      item.innerHTML = `
        <span class="drag-handle">::</span>
        <span class="admin-link-main">
          <span class="admin-link-title"></span>
          <span class="admin-link-url"></span>
        </span>
        <label class="admin-hidden-toggle">
          <input type="checkbox" data-action="toggle-hidden">
          <span></span>
        </label>
        <button type="button" class="text-button" data-action="edit"></button>
        <button type="button" class="text-button danger-button" data-action="delete"></button>
      `;
      item.querySelector(".admin-link-title").textContent = link.title;
      item.querySelector(".admin-link-url").textContent = link.url;
      item.querySelector('[data-action="toggle-hidden"]').checked = Boolean(link.hidden);
      item.querySelector(".admin-hidden-toggle span").textContent = t("fieldIsHidden", "Hidden");
      item.querySelector('[data-action="edit"]').textContent = t("buttonEdit", "Edit");
      item.querySelector('[data-action="delete"]').textContent = t("buttonDelete", "Delete");
      list.appendChild(item);
    });
    if (empty) empty.hidden = visibleLinks.length > 0;
  };

  const renderCategoryFilter = () => {
    if (!categoryFilter) return;
    const currentValue = categoryFilter.value;
    const categories = categoriesFromLinks();
    categoryFilter.innerHTML = "";
    const allOption = document.createElement("option");
    allOption.value = "";
    allOption.textContent = t("categoryFilterAll", "All boxes");
    categoryFilter.appendChild(allOption);
    categories.forEach((category) => {
      const option = document.createElement("option");
      option.value = category.value;
      option.textContent = category.label;
      categoryFilter.appendChild(option);
    });
    categoryFilter.value = categories.some((category) => category.value === currentValue) ? currentValue : "";
  };

  const categoriesFromLinks = () => {
    return categories.map((category) => ({
      value: category.name,
      label: category.name,
      count: category.count,
      isDefault: Boolean(category.isDefault)
    }));
  };

  const categoryComboboxes = () => Array.from(document.querySelectorAll("[data-category-combobox]"));

  const categoryComboboxParts = (target) => ({
    hidden: target.querySelector('input[name="category"]'),
    toggle: target.querySelector("[data-category-toggle]"),
    menu: target.querySelector("[data-category-menu]"),
    options: target.querySelector("[data-category-options]"),
    newInput: target.querySelector("[data-category-new]")
  });

  const categoryPlaceholder = () => t("categoryPlaceholder", "Select or create a box");

  const resetCategoryMenuPosition = (menu) => {
    if (!menu) return;
    menu.style.left = "";
    menu.style.top = "";
    menu.style.width = "";
    menu.style.maxHeight = "";
  };

  const positionCategoryMenu = (target) => {
    const { toggle, menu } = categoryComboboxParts(target);
    if (!toggle || !menu || menu.hidden) return;

    const rect = toggle.getBoundingClientRect();
    const gap = 6;
    const margin = 12;
    const belowSpace = window.innerHeight - rect.bottom - margin;
    const aboveSpace = rect.top - margin;
    const openAbove = belowSpace < 180 && aboveSpace > belowSpace;
    const availableSpace = openAbove ? aboveSpace : belowSpace;
    const maxHeight = Math.max(160, Math.min(280, availableSpace - gap));

    menu.style.left = `${rect.left}px`;
    menu.style.width = `${rect.width}px`;
    menu.style.maxHeight = `${maxHeight}px`;
    menu.style.top = openAbove
      ? `${Math.max(margin, rect.top - maxHeight - gap)}px`
      : `${rect.bottom + gap}px`;
  };

  const closeCategoryMenus = (except) => {
    categoryComboboxes().forEach((target) => {
      if (target === except) return;
      const { menu } = categoryComboboxParts(target);
      if (menu) {
        menu.hidden = true;
        resetCategoryMenuPosition(menu);
      }
    });
    if (!except) activeCategoryCombobox = null;
  };

  const setCategoryComboboxValue = (target, value) => {
    const { hidden, toggle, newInput } = categoryComboboxParts(target);
    const nextValue = String(value || "").trim();
    if (hidden) hidden.value = nextValue;
    if (toggle) toggle.textContent = nextValue || categoryPlaceholder();
    const knownCategory = categoriesFromLinks().some((category) => category.value === nextValue);
    if (newInput) newInput.value = nextValue && !knownCategory ? nextValue : "";
  };

  const setFormCategory = (form, value) => {
    const target = form.querySelector("[data-category-combobox]");
    if (target) setCategoryComboboxValue(target, value);
  };

  const renderCategoryCombobox = (target) => {
    const { hidden, options } = categoryComboboxParts(target);
    if (!options) return;
    const currentValue = hidden ? hidden.value : "";
    options.innerHTML = "";
    categoriesFromLinks().forEach((category) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "category-combobox-option";
      button.dataset.categoryValue = category.value;
      button.textContent = category.label;
      button.classList.toggle("is-active", category.value === currentValue);
      options.appendChild(button);
    });
    setCategoryComboboxValue(target, currentValue);
    positionCategoryMenu(target);
  };

  const renderCategoryComboboxes = () => {
    categoryComboboxes().forEach(renderCategoryCombobox);
  };

  const settingsPayloadFromForms = () => {
    const settingsData = new FormData(settingsForm);
    const adminData = new FormData(adminSettingsForm);
    return {
      siteTitle: String(settingsData.get("siteTitle") || "").trim(),
      language: String(settingsData.get("language") || "zhTW"),
      defaultTheme: String(settingsData.get("defaultTheme") || "light"),
      openNewTab: settingsData.get("openNewTab") === "on",
      background: String(settingsData.get("background") || "").trim(),
      searchEngineURL: String(settingsData.get("searchEngineURL") || "").trim(),
      adminUsername: String(adminData.get("adminUsername") || "").trim(),
      newPassword: String(adminData.get("newPassword") || "")
    };
  };

  const applySettingsResponse = (settings) => {
    if (window.applySharedTheme) {
      window.applySharedTheme(settings.defaultTheme || "light");
    } else {
      document.documentElement.dataset.theme = settings.defaultTheme || "light";
    }
    if (settings.background) {
      document.body.style.setProperty("--page-bg-image", `url("${settings.background}")`);
    } else {
      document.body.style.removeProperty("--page-bg-image");
    }
  };

  const saveSettings = async () => {
    const settings = await requestJSON("/api/admin/settings", {
      method: "PUT",
      body: JSON.stringify(settingsPayloadFromForms())
    });
    adminSettingsForm.elements.newPassword.value = "";
    return settings;
  };

  const renderBoxes = () => {
    const categories = categoriesFromLinks();
    if (boxCount) boxCount.textContent = String(categories.length);
    if (boxEmpty) boxEmpty.hidden = categories.length > 0;
    renderBoxList(boxList, categories, true);
    renderBoxList(boxAsideList, categories, false);
  };

  const renderSearchEngines = () => {
    if (!searchEngineList) return;
    searchEngineList.innerHTML = "";
    if (searchEngineCount) searchEngineCount.textContent = String(searchEngines.length);
    if (searchEngineEmpty) searchEngineEmpty.hidden = searchEngines.length > 0;

    searchEngines.forEach((engine) => {
      const item = document.createElement("li");
      item.dataset.searchEngineId = String(engine.id);
      item.innerHTML = `
        <span class="drag-handle search-engine-drag-handle">::</span>
        <span class="admin-link-main">
          <span class="admin-link-title"></span>
          <span class="admin-link-url"></span>
        </span>
        <label class="admin-hidden-toggle">
          <input type="checkbox" data-action="toggle-search-engine">
          <span></span>
        </label>
        <button type="button" class="text-button" data-action="edit-search-engine"></button>
        <button type="button" class="text-button danger-button" data-action="delete-search-engine"></button>
      `;
      item.querySelector(".admin-link-title").textContent = engine.name;
      item.querySelector(".admin-link-url").textContent = engine.url;
      item.querySelector('[data-action="toggle-search-engine"]').checked = Boolean(engine.enabled);
      item.querySelector(".admin-hidden-toggle span").textContent = t("fieldSearchEngineEnabled", "Enabled");
      item.querySelector('[data-action="edit-search-engine"]').textContent = t("buttonEdit", "Edit");
      item.querySelector('[data-action="delete-search-engine"]').textContent = t("buttonDelete", "Delete");
      searchEngineList.appendChild(item);
    });
  };

  const renderBoxList = (target, categories, detailed) => {
    if (!target) return;
    target.innerHTML = "";
    categories.forEach((category) => {
      const item = document.createElement("li");
      item.dataset.category = category.value;
      item.className = `box-item${category.isDefault ? " is-default" : ""}`;
      item.innerHTML = detailed
        ? `
          <span class="drag-handle box-drag-handle">::</span>
          <span class="box-name"></span>
          <strong></strong>
          <span class="box-actions"></span>
        `
        : `<span></span><span class="status-pill"></span>`;
      if (detailed) {
        item.querySelector(".box-name").textContent = category.label;
      } else {
        item.querySelector("span").textContent = category.label;
      }
      if (detailed) {
        item.querySelector("strong").textContent = String(category.count);
        const actions = item.querySelector(".box-actions");
        if (!category.isDefault) {
          actions.innerHTML = `
            <button type="button" class="text-button" data-action="rename-category"></button>
            <button type="button" class="text-button danger-button" data-action="delete-category"></button>
          `;
          actions.querySelector('[data-action="rename-category"]').textContent = t("buttonEdit", "Edit");
          actions.querySelector('[data-action="delete-category"]').textContent = t("buttonDelete", "Delete");
        }
      } else {
        item.querySelector(".status-pill").textContent = String(category.count);
      }
      if (!detailed) {
        item.addEventListener("click", () => {
          if (categoryFilter) {
            categoryFilter.value = category.value;
            renderLinks();
          }
          switchView("links");
        });
      }
      target.appendChild(item);
    });
  };

  const currentOrderedLinks = () => {
    return [...links].sort((left, right) => {
      if (left.sortOrder !== right.sortOrder) {
        return left.sortOrder - right.sortOrder;
      }
      return left.id - right.id;
    });
  };

  const buildSortPayload = () => {
    const draggedIds = Array.from(list.querySelectorAll("[data-id]")).map((item) => Number(item.dataset.id));
    const selectedCategory = categoryFilter ? categoryFilter.value : "";
    let orderedLinks;

    if (selectedCategory) {
      const draggedLinks = draggedIds.map((id) => links.find((link) => link.id === id)).filter(Boolean);
      let draggedIndex = 0;
      orderedLinks = currentOrderedLinks().map((link) => {
        if (link.category !== selectedCategory) return link;
        const replacement = draggedLinks[draggedIndex];
        draggedIndex += 1;
        return replacement || link;
      });
    } else {
      orderedLinks = draggedIds.map((id) => links.find((link) => link.id === id)).filter(Boolean);
    }

    return orderedLinks.map((link, index) => ({
      id: link.id,
      sortOrder: index + 1
    }));
  };

  const buildCategorySortPayload = () => {
    return Array.from(boxList.querySelectorAll("[data-category]:not(.is-default)")).map((item, index) => ({
      name: item.dataset.category,
      sortOrder: index + 1
    }));
  };

  const buildSearchEngineSortPayload = () => {
    return Array.from(searchEngineList.querySelectorAll("[data-search-engine-id]")).map((item, index) => ({
      id: Number(item.dataset.searchEngineId),
      sortOrder: index + 1
    }));
  };

  const readLinkForm = (form) => {
    const formData = new FormData(form);
    const sortOrder = Number(formData.get("sortOrder"));
    return {
      id: Number(formData.get("id")) || 0,
      title: String(formData.get("title") || "").trim(),
      url: String(formData.get("url") || "").trim(),
      description: String(formData.get("description") || "").trim(),
      category: String(formData.get("category") || "").trim(),
      icon: String(formData.get("icon") || "").trim(),
      sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
      hidden: formData.get("hidden") === "on"
    };
  };

  const readSearchEngineForm = (form) => {
    const formData = new FormData(form);
    const sortOrder = Number(formData.get("sortOrder"));
    return {
      id: Number(formData.get("id")) || 0,
      name: String(formData.get("name") || "").trim(),
      url: String(formData.get("url") || "").trim(),
      sortOrder: Number.isFinite(sortOrder) ? sortOrder : 0,
      enabled: formData.get("enabled") === "on"
    };
  };

  const fillEditSearchEngineForm = (engine) => {
    editSearchEngineForm.elements.id.value = engine.id || "";
    editSearchEngineForm.elements.name.value = engine.name || "";
    editSearchEngineForm.elements.url.value = engine.url || "";
    editSearchEngineForm.elements.sortOrder.value = engine.sortOrder || "";
    editSearchEngineForm.elements.enabled.checked = Boolean(engine.enabled);
    openDialog(editSearchEngineDialog);
    editSearchEngineForm.elements.name.focus();
  };

  const clearEditSearchEngineForm = () => {
    editSearchEngineForm.reset();
    editSearchEngineForm.elements.id.value = "";
    editSearchEngineForm.elements.sortOrder.value = "";
    closeDialog(editSearchEngineDialog);
  };

  const fillEditForm = (link) => {
    editForm.elements.id.value = link.id || "";
    editForm.elements.title.value = link.title || "";
    editForm.elements.url.value = link.url || "";
    editForm.elements.description.value = link.description || "";
    setFormCategory(editForm, link.category || "");
    editForm.elements.icon.value = link.icon || "";
    editForm.elements.sortOrder.value = link.sortOrder || "";
    editForm.elements.hidden.checked = Boolean(link.hidden);
    openDialog(editDialog);
    editForm.elements.title.focus();
  };

  const clearCreateForm = () => {
    createForm.reset();
    const selectedCategory = categoryFilter ? categoryFilter.value : "";
    setFormCategory(createForm, selectedCategory);
  };

  const clearEditForm = () => {
    editForm.reset();
    editForm.elements.id.value = "";
    setFormCategory(editForm, "");
    closeDialog(editDialog);
  };

  createForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(message, "");
    try {
      await requestJSON("/api/admin/links", {
        method: "POST",
        body: JSON.stringify(readLinkForm(createForm))
      });
      clearCreateForm();
      closeDialog(createDialog);
      await loadLinks();
      showMessage(message, t("messageCardCreated", "Card created"));
    } catch (error) {
      showMessage(message, error.message, true);
    }
  });

  categoryForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(categoryMessage, "");
    const formData = new FormData(categoryForm);
    try {
      categories = await requestJSON("/api/admin/categories", {
        method: "POST",
        body: JSON.stringify({
          name: String(formData.get("name") || "").trim()
        })
      });
      categoryForm.reset();
      await loadLinks();
      showMessage(categoryMessage, t("messageCategoryCreated", "Box created"));
    } catch (error) {
      showMessage(categoryMessage, error.message, true);
    }
  });

  searchEngineForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(searchEngineMessage, "");
    try {
      await requestJSON("/api/admin/search-engines", {
        method: "POST",
        body: JSON.stringify(readSearchEngineForm(searchEngineForm))
      });
      searchEngineForm.reset();
      searchEngineForm.elements.enabled.checked = true;
      await loadLinks();
      showMessage(searchEngineMessage, t("messageSearchEngineCreated", "Search engine created"));
    } catch (error) {
      showMessage(searchEngineMessage, error.message, true);
    }
  });

  editForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(message, "");
    const link = readLinkForm(editForm);
    try {
      await requestJSON(`/api/admin/links/${link.id}`, {
        method: "PUT",
        body: JSON.stringify(link)
      });
      clearEditForm();
      await loadLinks();
      showMessage(message, t("messageCardUpdated", "Card updated"));
    } catch (error) {
      showMessage(message, error.message, true);
    }
  });

  editSearchEngineForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(searchEngineMessage, "");
    const engine = readSearchEngineForm(editSearchEngineForm);
    try {
      await requestJSON(`/api/admin/search-engines/${engine.id}`, {
        method: "PUT",
        body: JSON.stringify(engine)
      });
      clearEditSearchEngineForm();
      await loadLinks();
      showMessage(searchEngineMessage, t("messageSearchEngineUpdated", "Search engine updated"));
    } catch (error) {
      showMessage(searchEngineMessage, error.message, true);
    }
  });

  document.querySelectorAll('[data-action="open-create"]').forEach((button) => {
    button.addEventListener("click", () => {
      clearCreateForm();
      showMessage(message, "");
      openDialog(createDialog);
      createForm.elements.title.focus();
    });
  });

  document.querySelector('[data-action="cancel-create"]')?.addEventListener("click", () => {
    clearCreateForm();
    closeDialog(createDialog);
    showMessage(message, "");
  });

  document.querySelector('[data-action="cancel-edit"]')?.addEventListener("click", () => {
    clearEditForm();
    showMessage(message, "");
  });

  document.querySelector('[data-action="cancel-search-engine-edit"]')?.addEventListener("click", () => {
    clearEditSearchEngineForm();
    showMessage(searchEngineMessage, "");
  });

  document.addEventListener("click", (event) => {
    if (event.target.closest("[data-category-combobox]")) return;
    closeCategoryMenus();
  });

  document.addEventListener("click", (event) => {
    const toggle = event.target.closest("[data-category-toggle]");
    if (toggle) {
      const target = toggle.closest("[data-category-combobox]");
      const { menu } = categoryComboboxParts(target);
      if (!menu) return;
      const willOpen = menu.hidden;
      closeCategoryMenus(target);
      renderCategoryCombobox(target);
      menu.hidden = !willOpen;
      activeCategoryCombobox = willOpen ? target : null;
      positionCategoryMenu(target);
      return;
    }

    const option = event.target.closest("[data-category-value]");
    if (!option) return;
    const target = option.closest("[data-category-combobox]");
    const { menu, toggle: optionToggle } = categoryComboboxParts(target);
    setCategoryComboboxValue(target, option.dataset.categoryValue || "");
    renderCategoryCombobox(target);
    if (menu) {
      menu.hidden = true;
      resetCategoryMenuPosition(menu);
    }
    activeCategoryCombobox = null;
    optionToggle?.focus();
  });

  document.addEventListener("input", (event) => {
    const input = event.target.closest("[data-category-new]");
    if (!input) return;
    const target = input.closest("[data-category-combobox]");
    const { hidden, toggle } = categoryComboboxParts(target);
    const value = input.value.trim();
    if (hidden) hidden.value = value;
    if (toggle) toggle.textContent = value || categoryPlaceholder();
    renderCategoryCombobox(target);
    input.focus();
  });

  document.addEventListener("keydown", (event) => {
    const input = event.target.closest("[data-category-new]");
    if (!input || event.key !== "Enter") return;
    event.preventDefault();
    const target = input.closest("[data-category-combobox]");
    const { menu, toggle } = categoryComboboxParts(target);
    if (menu) {
      menu.hidden = true;
      resetCategoryMenuPosition(menu);
    }
    activeCategoryCombobox = null;
    toggle?.focus();
  });

  window.addEventListener("resize", () => {
    if (activeCategoryCombobox) positionCategoryMenu(activeCategoryCombobox);
  });

  window.addEventListener("scroll", () => {
    if (activeCategoryCombobox) positionCategoryMenu(activeCategoryCombobox);
  }, true);

  boxList?.addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const item = button.closest("[data-category]");
    if (!item) return;
    const name = item.dataset.category || "";
    const category = categories.find((candidate) => candidate.name === name);
    if (!category || category.isDefault) return;

    if (button.dataset.action === "rename-category") {
      const nextName = window.prompt(t("promptRenameCategory", "Rename box"), category.name);
      if (nextName === null) return;
      if (nextName.trim() === category.name) return;
      try {
        categories = await requestJSON(`/api/admin/categories/${encodeURIComponent(category.name)}`, {
          method: "PUT",
          body: JSON.stringify({ name: nextName.trim() })
        });
        await loadLinks();
        showMessage(message, t("messageCategoryUpdated", "Box renamed"));
      } catch (error) {
        showMessage(message, error.message, true);
      }
      return;
    }

    if (!window.confirm(`${t("confirmDeleteCategoryPrefix", "Delete box \"")}${category.name}${t("confirmDeleteCategorySuffix", "\"? Cards will move to Uncategorized.")}`)) return;
    try {
      categories = await requestJSON(`/api/admin/categories/${encodeURIComponent(category.name)}`, {
        method: "DELETE"
      });
      await loadLinks();
      showMessage(message, t("messageCategoryDeleted", "Box deleted"));
    } catch (error) {
      showMessage(message, error.message, true);
    }
  });

  searchEngineList?.addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;

    const item = button.closest("[data-search-engine-id]");
    const id = Number(item.dataset.searchEngineId);
    const engine = searchEngines.find((candidate) => candidate.id === id);
    if (!engine) return;

    if (button.dataset.action === "edit-search-engine") {
      fillEditSearchEngineForm(engine);
      showMessage(searchEngineMessage, "");
      return;
    }

    if (!window.confirm(`${t("confirmDeleteSearchEnginePrefix", "Delete search engine \"")}${engine.name}${t("confirmDeleteSearchEngineSuffix", "\"?")}`)) return;
    try {
      await requestJSON(`/api/admin/search-engines/${id}`, {
        method: "DELETE"
      });
      await loadLinks();
      showMessage(searchEngineMessage, t("messageSearchEngineDeleted", "Search engine deleted"));
    } catch (error) {
      showMessage(searchEngineMessage, error.message, true);
    }
  });

  searchEngineList?.addEventListener("change", async (event) => {
    const input = event.target.closest('input[data-action="toggle-search-engine"]');
    if (!input) return;

    const item = input.closest("[data-search-engine-id]");
    const id = Number(item.dataset.searchEngineId);
    const engine = searchEngines.find((candidate) => candidate.id === id);
    if (!engine) return;

    const nextEnabled = input.checked;
    try {
      await requestJSON(`/api/admin/search-engines/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          ...engine,
          enabled: nextEnabled
        })
      });
      await loadLinks();
      showMessage(searchEngineMessage, t("messageSearchEngineUpdated", "Search engine updated"));
    } catch (error) {
      input.checked = !nextEnabled;
      showMessage(searchEngineMessage, error.message, true);
    }
  });

  if (categoryFilter) {
    categoryFilter.addEventListener("change", renderLinks);
  }

  list.addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;

    const item = button.closest("[data-id]");
    const id = Number(item.dataset.id);
    const link = links.find((candidate) => candidate.id === id);
    if (!link) return;

    if (button.dataset.action === "edit") {
      fillEditForm(link);
      showMessage(message, "");
      return;
    }

    if (!window.confirm(`${t("confirmDeletePrefix", "Delete \"")}${link.title}${t("confirmDeleteSuffix", "\"?")}`)) return;
    try {
      await requestJSON(`/api/admin/links/${id}`, {
        method: "DELETE"
      });
      await loadLinks();
      showMessage(message, t("messageCardDeleted", "Card deleted"));
    } catch (error) {
      showMessage(message, error.message, true);
    }
  });

  list.addEventListener("change", async (event) => {
    const input = event.target.closest('input[data-action="toggle-hidden"]');
    if (!input) return;

    const item = input.closest("[data-id]");
    const id = Number(item.dataset.id);
    const link = links.find((candidate) => candidate.id === id);
    if (!link) return;

    const nextHidden = input.checked;
    try {
      await requestJSON(`/api/admin/links/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          ...link,
          hidden: nextHidden
        })
      });
      await loadLinks();
      showMessage(message, t("messageCardUpdated", "Card updated"));
    } catch (error) {
      input.checked = !nextHidden;
      showMessage(message, error.message, true);
    }
  });

  settingsForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(settingsMessage, "");
    try {
      const currentLanguage = document.documentElement.lang || "";
      const settings = await saveSettings();
      applySettingsResponse(settings);
      if (settings.language && settings.language !== currentLanguage) {
        window.location.reload();
        return;
      }
      showMessage(settingsMessage, t("messageSettingsUpdated", "Settings saved"));
    } catch (error) {
      showMessage(settingsMessage, error.message, true);
    }
  });

  adminSettingsForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    showMessage(adminSettingsMessage, "");
    try {
      const settings = await saveSettings();
      applySettingsResponse(settings);
      showMessage(adminSettingsMessage, t("messageSettingsUpdated", "Settings saved"));
    } catch (error) {
      showMessage(adminSettingsMessage, error.message, true);
    }
  });

  document.querySelectorAll("[data-export-scope]").forEach((button) => {
    button.addEventListener("click", async () => {
      const scope = button.dataset.exportScope || "";
      showMessage(importExportMessage, "");
      showOperationOverlay(t("messageExportProcessing", "Exporting..."));
      try {
        const response = await requestDownload(`/api/admin/export/${scope}`);
        const blob = await response.blob();
        const filename = filenameFromResponse(response, `sam-nav-${scope}.json`);
        downloadBlob(blob, filename);
        const completeMessage = t("messageExportDownloaded", "Export downloaded");
        hideOperationOverlay();
        await showCompleteDialog(completeMessage);
        showMessage(importExportMessage, completeMessage);
      } catch (error) {
        hideOperationOverlay();
        showMessage(importExportMessage, error.message, true);
      }
    });
  });

  document.querySelectorAll("[data-import-scope]").forEach((button) => {
    button.addEventListener("click", () => {
      const scope = button.dataset.importScope || "";
      const input = document.querySelector(`[data-import-input="${scope}"]`);
      if (!input) return;
      input.click();
    });
  });

  document.querySelectorAll("[data-import-input]").forEach((input) => {
    input.addEventListener("change", async () => {
      const scope = input.dataset.importInput || "";
      const file = input.files && input.files[0];
      if (!file) {
        showMessage(importExportMessage, t("errorNoImportFile", "Select a JSON file first"), true);
        return;
      }
      if (!window.confirm(`${t("confirmImportPrefix", "Import ")}${file.name}${t("confirmImportSuffix", "? Existing data in this range will be replaced.")}`)) {
        input.value = "";
        return;
      }
      showMessage(importExportMessage, "");
      showOperationOverlay(t("messageImportProcessing", "Importing..."));
      try {
        const content = await file.text();
        const settings = await requestJSON(`/api/admin/import/${scope}`, {
          method: "POST",
          body: content
        });
        input.value = "";
        await loadLinks();
        const completeMessage = t("messageImportCompleted", "Import completed");
        hideOperationOverlay();
        if (scope === "settings" || scope === "all") {
          if (window.applySharedTheme) {
            window.applySharedTheme(settings.defaultTheme || "light");
          } else {
            document.documentElement.dataset.theme = settings.defaultTheme || "light";
          }
          await showCompleteDialog(completeMessage);
          window.location.reload();
          return;
        }
        await showCompleteDialog(completeMessage);
        showMessage(importExportMessage, completeMessage);
      } catch (error) {
        input.value = "";
        hideOperationOverlay();
        showMessage(importExportMessage, error.message, true);
      }
    });
  });

  if (window.Sortable) {
    window.Sortable.create(list, {
      animation: 150,
      handle: ".drag-handle",
      onEnd: async function () {
        const items = buildSortPayload();

        try {
          await requestJSON("/api/admin/links/sort", {
            method: "PUT",
            body: JSON.stringify({ items })
          });
          showMessage(message, t("sortLog", "Order updated"));
          await loadLinks();
        } catch (error) {
          showMessage(message, error.message, true);
          await loadLinks();
        }
      }
    });

    if (boxList) {
      window.Sortable.create(boxList, {
        animation: 150,
        draggable: ".box-item:not(.is-default)",
        handle: ".box-drag-handle",
        onEnd: async function () {
          const items = buildCategorySortPayload();

          try {
            categories = await requestJSON("/api/admin/categories/sort", {
              method: "PUT",
              body: JSON.stringify({ items })
            });
            await loadLinks();
            showMessage(message, t("sortLog", "Order updated"));
          } catch (error) {
            showMessage(message, error.message, true);
            await loadLinks();
          }
        }
      });
    }

    if (searchEngineList) {
      window.Sortable.create(searchEngineList, {
        animation: 150,
        handle: ".search-engine-drag-handle",
        onEnd: async function () {
          const items = buildSearchEngineSortPayload();

          try {
            await requestJSON("/api/admin/search-engines/sort", {
              method: "PUT",
              body: JSON.stringify({ items })
            });
            await loadLinks();
            showMessage(searchEngineMessage, t("sortLog", "Order updated"));
          } catch (error) {
            showMessage(searchEngineMessage, error.message, true);
            await loadLinks();
          }
        }
      });
    }
  }

  loadLinks().catch((error) => {
    showMessage(message, error.message, true);
  });
})();

(function () {
  const list = document.getElementById("sortable-links");
  if (!list || !window.Sortable) return;

  window.Sortable.create(list, {
    animation: 150,
    handle: ".drag-handle",
    onEnd: function () {
      const items = Array.from(list.querySelectorAll("[data-id]")).map((item, index) => ({
        id: item.dataset.id,
        sortOrder: index + 1
      }));

      console.log(list.dataset.sortLog, items);
    }
  });
})();

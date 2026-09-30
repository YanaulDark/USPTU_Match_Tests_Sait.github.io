(function () {
  const KEY = 'lecturehub-theme';
  const root = document.documentElement;

  // Восстанавливаем сохранённую тему (если есть)
  const saved = localStorage.getItem(KEY);
  if (saved === 'light' || saved === 'dark') {
    root.setAttribute('data-theme', saved);
  }
  // Если ничего не сохранено — работает @media (prefers-color-scheme)
  // из CSS, и data-theme не выставляется.

  function currentTheme() {
    const set = root.getAttribute('data-theme');
    if (set) return set;
    return window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark' : 'light';
  }

  function apply(theme) {
    root.setAttribute('data-theme', theme);
    localStorage.setItem(KEY, theme);
  }

  window.toggleTheme = function () {
    apply(currentTheme() === 'dark' ? 'light' : 'dark');
  };

  // Проставляем кнопкам обработчики, когда DOM готов
  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('.theme-toggle').forEach(btn => {
      btn.addEventListener('click', window.toggleTheme);
    });
  });
})();

// ccmanager — the only script: command palette keys and copy buttons.
(() => {
  const palette = document.getElementById('palette');
  const input = palette && palette.querySelector('.palette__input');
  const enabled = '[role="option"]:not([aria-disabled="true"])';

  function options() {
    return [...palette.querySelectorAll(enabled)];
  }

  function select(option) {
    for (const o of palette.querySelectorAll('[aria-selected="true"]')) o.setAttribute('aria-selected', 'false');
    if (!option) return input.removeAttribute('aria-activedescendant');
    option.setAttribute('aria-selected', 'true');
    input.setAttribute('aria-activedescendant', option.id);
    option.scrollIntoView({ block: 'nearest' });
  }

  if (palette) {
    document.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        if (palette.open) return palette.close();
        palette.showModal();
        input.select();
      }
    });

    palette.addEventListener('keydown', (e) => {
      const list = options();
      const i = list.findIndex((o) => o.getAttribute('aria-selected') === 'true');
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        const step = e.key === 'ArrowDown' ? 1 : -1;
        const next = i === -1 ? (step > 0 ? 0 : list.length - 1) : (i + step + list.length) % list.length;
        select(list[next]);
      } else if (e.key === 'Enter' && list[i]) {
        e.preventDefault();
        list[i].click();
      }
    });

    palette.addEventListener('pointermove', (e) => {
      const o = e.target.closest(enabled);
      if (o && o.getAttribute('aria-selected') !== 'true') select(o);
    });

    // Fresh results from hx-get: highlight the first match.
    palette.addEventListener('htmx:afterSettle', (e) => {
      if (e.target.id === 'palette-results') select(options()[0]);
    });
  }

  // Copies the resulting text: deleted lines, hunk headers and line numbers are left out.
  document.addEventListener('click', async (e) => {
    const btn = e.target.closest('[data-copy]');
    if (!btn) return;
    const el = document.getElementById(btn.dataset.copy);
    const lines = el.querySelectorAll('.code-line:not(.code-line--del):not(.code-line--hunk)');
    const text = lines.length ? [...lines].map((l) => l.textContent).join('\n') : el.textContent.trim();
    await navigator.clipboard.writeText(text);
    btn.dataset.copied = '';
    setTimeout(() => delete btn.dataset.copied, 1200);
  });
})();

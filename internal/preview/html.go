package preview

import "html/template"

// indexTemplate is the embedded HTML page served at /.
// It includes a slide navigator, SSE hot-reload, and keyboard navigation.
var indexTemplate = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Slide Preview</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { background: #1a1a2e; color: #e0e0e0; font-family: system-ui, sans-serif; display: flex; flex-direction: column; height: 100vh; overflow: hidden; }
  header { display: flex; align-items: center; gap: 12px; padding: 8px 16px; background: #16213e; border-bottom: 1px solid #0f3460; flex-shrink: 0; }
  header h1 { font-size: 14px; font-weight: 600; color: #e94560; }
  header .info { font-size: 12px; color: #888; }
  header .status { margin-left: auto; font-size: 11px; padding: 3px 8px; border-radius: 4px; }
  header .status.connected { background: #1b4332; color: #52b788; }
  header .status.disconnected { background: #532b2b; color: #e94560; }
  .main { display: flex; flex: 1; overflow: hidden; }
  .sidebar { width: 180px; background: #16213e; border-right: 1px solid #0f3460; overflow-y: auto; flex-shrink: 0; padding: 8px; }
  .thumb { cursor: pointer; border: 2px solid transparent; border-radius: 4px; margin-bottom: 6px; overflow: hidden; transition: border-color 0.15s; }
  .thumb.active { border-color: #e94560; }
  .thumb:hover { border-color: #0f3460; }
  .thumb img { width: 100%; display: block; }
  .thumb .label { text-align: center; font-size: 10px; padding: 2px 0; color: #888; }
  .viewer { flex: 1; display: flex; align-items: center; justify-content: center; overflow: hidden; padding: 16px; }
  .viewer img { max-width: 100%; max-height: 100%; object-fit: contain; border-radius: 4px; box-shadow: 0 4px 24px rgba(0,0,0,0.5); }
  .errors { background: #3b0d0d; color: #ff6b6b; padding: 8px 16px; font-size: 12px; font-family: monospace; white-space: pre-wrap; border-top: 1px solid #5c1a1a; max-height: 120px; overflow-y: auto; flex-shrink: 0; }
  footer { padding: 4px 16px; background: #16213e; border-top: 1px solid #0f3460; font-size: 11px; color: #555; flex-shrink: 0; display: flex; gap: 16px; }
</style>
</head>
<body>
<header>
  <h1>Slide Preview</h1>
  <span class="info" id="file">{{.File}}</span>
  <span class="status disconnected" id="status">connecting&hellip;</span>
</header>
<div class="main">
  <div class="sidebar" id="sidebar"></div>
  <div class="viewer"><img id="slide" alt="slide"></div>
</div>
<div class="errors" id="errors" style="display:none"></div>
<footer>
  <span id="counter">0 / 0</span>
  <span>&larr; &rarr; or click thumbnails</span>
</footer>
<script>
(function() {
  let current = 0;
  let total = 0;
  let generation = 0;

  const slideImg = document.getElementById('slide');
  const sidebar = document.getElementById('sidebar');
  const counter = document.getElementById('counter');
  const statusEl = document.getElementById('status');
  const errorsEl = document.getElementById('errors');

  function showSlide(n) {
    if (n < 0 || n >= total) return;
    current = n;
    slideImg.src = '/slide/' + n + '.png?g=' + generation;
    counter.textContent = (n + 1) + ' / ' + total;
    document.querySelectorAll('.thumb').forEach(function(el, i) {
      el.classList.toggle('active', i === n);
    });
    // Scroll active thumbnail into view
    const active = sidebar.querySelector('.thumb.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }

  function buildThumbnails() {
    sidebar.innerHTML = '';
    for (let i = 0; i < total; i++) {
      const div = document.createElement('div');
      div.className = 'thumb' + (i === current ? ' active' : '');
      div.innerHTML = '<img src="/slide/' + i + '.png?g=' + generation + '" alt="Slide ' + (i+1) + '"><div class="label">' + (i+1) + '</div>';
      div.onclick = (function(idx) { return function() { showSlide(idx); }; })(i);
      sidebar.appendChild(div);
    }
  }

  // Keyboard navigation
  document.addEventListener('keydown', function(e) {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown' || e.key === ' ') {
      e.preventDefault();
      showSlide(current + 1);
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      showSlide(current - 1);
    } else if (e.key === 'Home') {
      showSlide(0);
    } else if (e.key === 'End') {
      showSlide(total - 1);
    }
  });

  // SSE hot reload
  function connect() {
    const es = new EventSource('/events');
    es.onopen = function() {
      statusEl.textContent = 'connected';
      statusEl.className = 'status connected';
    };
    es.addEventListener('reload', function(e) {
      const data = JSON.parse(e.data);
      total = data.slides;
      generation = data.generation;
      if (current >= total) current = Math.max(0, total - 1);
      buildThumbnails();
      showSlide(current);
      if (data.error) {
        errorsEl.textContent = data.error;
        errorsEl.style.display = 'block';
      } else {
        errorsEl.style.display = 'none';
      }
    });
    es.onerror = function() {
      statusEl.textContent = 'disconnected';
      statusEl.className = 'status disconnected';
      es.close();
      setTimeout(connect, 2000);
    };
  }
  connect();
})();
</script>
</body>
</html>`))

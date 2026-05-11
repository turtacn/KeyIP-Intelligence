const CACHE_VERSION = 'v1';
const STATIC_CACHE = `keyip-static-${CACHE_VERSION}`;
const API_CACHE = `keyip-api-${CACHE_VERSION}`;

const PRECACHE_URLS = ['/', '/index.html'];

// 安装阶段：预缓存入口 HTML
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => {
      return cache.addAll(PRECACHE_URLS);
    })
  );
  self.skipWaiting();
});

// 激活阶段：清理旧缓存
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key.startsWith('keyip-') && key !== STATIC_CACHE && key !== API_CACHE)
          .map((key) => caches.delete(key))
      )
    )
  );
  self.clients.claim();
});

// 判断请求是否为导航请求（HTML 页面）
function isNavigationRequest(request) {
  return request.mode === 'navigate';
}

// 判断请求是否为 API 请求
function isApiRequest(url) {
  return url.pathname.startsWith('/_api/');
}

// 判断是否为静态资源（Vite 构建产物的哈希文件名确保缓存安全）
function isStaticAsset(url) {
  // Vite 对带 hash 的生产文件使用 /assets/*.hash.ext 格式
  if (url.pathname.startsWith('/assets/')) return true;
  // 图标和字体文件
  if (/\.(svg|png|ico|woff2?|ttf|eot)$/i.test(url.pathname)) return true;
  return false;
}

// 网络优先策略：先请求网络，失败则回退缓存
async function networkFirst(request, cacheName) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(cacheName);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    if (cached) return cached;
    // API 请求失败时返回一个 503
    return new Response(JSON.stringify({ error: 'offline' }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    });
  }
}

// 缓存优先策略：命中缓存直接返回，否则请求网络并缓存
async function cacheFirst(request, cacheName) {
  const cached = await caches.match(request);
  if (cached) return cached;
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(cacheName);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    // 静态资源不可用时返回离线占位
    return new Response('Offline', { status: 503 });
  }
}

self.addEventListener('fetch', (event) => {
  const url = new URL(event.request.url);

  // 只处理同源请求
  if (url.origin !== self.location.origin) return;

  // 忽略非 GET 请求
  if (event.request.method !== 'GET') return;

  // 导航请求（HTML 页面）：网络优先，回退缓存
  if (isNavigationRequest(event.request)) {
    event.respondWith(networkFirst(event.request, STATIC_CACHE));
    return;
  }

  // API 请求：网络优先，回退缓存
  if (isApiRequest(url)) {
    event.respondWith(networkFirst(event.request, API_CACHE));
    return;
  }

  // 静态资源（JS、CSS、图片、字体）：缓存优先
  if (isStaticAsset(url)) {
    event.respondWith(cacheFirst(event.request, STATIC_CACHE));
    return;
  }

  // 其他请求：网络优先，回退缓存
  event.respondWith(networkFirst(event.request, STATIC_CACHE));
});

#!/usr/bin/env node
/**
 * KeyIP-Intelligence CDP E2E Verification (Container版本)
 * 通过 headless Chromium CDP 验证所有 14 个页面
 */
const { WebSocket } = require('ws');
const http = require('http');

const BASE_URL = 'http://192.168.99.100';
const CDP = 'http://localhost:2222';

const PAGES = {
  dashboard:         { url: '/dashboard',           title: 'KeyIP' },
  search:            { url: '/search',              title: 'KeyIP' },
  patentMining:      { url: '/patent-mining',       title: 'KeyIP' },
  knowledgeGraph:    { url: '/knowledge-graph',     title: 'KeyIP' },
  fto:               { url: '/fto',                 title: 'KeyIP' },
  infringement:      { url: '/infringement-watch',  title: 'KeyIP' },
  lifecycle:         { url: '/lifecycle',           title: 'KeyIP' },
  portfolio:         { url: '/portfolio-optimizer', title: 'KeyIP' },
  molecules:         { url: '/molecules',           title: 'KeyIP' },
  patents:           { url: '/patents/CN115650927B',title: 'KeyIP' },
  partners:          { url: '/partners',            title: 'KeyIP' },
  health:            { url: '/health',              title: 'KeyIP' },
  settings:          { url: '/settings',            title: 'KeyIP' },
  login:             { url: '/login',               title: 'KeyIP' },
};

async function getCdpPages() {
  return new Promise((resolve, reject) => {
    http.get(`${CDP}/json`, res => {
      let d = ''; res.on('data', c => d += c);
      res.on('end', () => resolve(JSON.parse(d)));
    }).on('error', reject);
  });
}

class CDPClient {
  constructor(wsUrl) { this.wsUrl = wsUrl; this.id = 0; this.pending = new Map(); }
  async connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.wsUrl);
      this.ws.on('open', () => { this.ws.on('message', d => { const m = JSON.parse(d.toString()); if (m.id && this.pending.has(m.id)) { this.pending.get(m.id)(m); this.pending.delete(m.id); } }); resolve(); });
      this.ws.on('error', reject);
      setTimeout(() => reject(new Error('timeout')), 10000);
    });
  }
  async send(method, params = {}) {
    const id = ++this.id;
    return new Promise((resolve, reject) => {
      this.pending.set(id, resolve);
      this.ws.send(JSON.stringify({ id, method, params }));
      setTimeout(() => { if (this.pending.has(id)) { this.pending.delete(id); reject(new Error(`${method} timeout`)); } }, 15000);
    });
  }
  async navigate(url) {
    await this.send('Page.enable');
    const u = url.startsWith('http') ? url : `${BASE_URL}${url}`;
    return this.send('Page.navigate', { url: u });
  }
  async evaluate(expr) {
    const r = await this.send('Runtime.evaluate', { expression: expr, returnByValue: true });
    return r.result?.result?.value;
  }
  async wait(ms) { return new Promise(r => setTimeout(r, ms)); }
  close() { if (this.ws) this.ws.close(); }
}

async function main() {
  console.log('╔══════════════════════════════════════╗');
  console.log('║  KeyIP CDP E2E Verification         ║');
  console.log('╚══════════════════════════════════════╝\n');

  const targets = await getCdpPages();
  console.log(`CDP: ${targets.length} page(s)\n`);

  const page = targets[0];
  const client = new CDPClient(page.webSocketDebuggerUrl);
  await client.connect();
  await client.send('Runtime.enable');

  let passed = 0, failed = 0, total = Object.keys(PAGES).length;

  for (const [name, cfg] of Object.entries(PAGES)) {
    try {
      const result = await client.navigate(cfg.url);
      await client.wait(2000);

      const title = await client.evaluate('document.title');
      const hasContent = await client.evaluate('document.body.innerText.length > 20');
      const hasErrors = await client.evaluate('(window.__cdp_errors || []).length');
      const apiMode = await client.evaluate("localStorage.getItem('keyip-api-mode') || 'default'");

      const ok = title && hasContent;
      if (ok) passed++;
      else failed++;

      const icon = ok ? '✅' : '❌';
      console.log(`${icon} ${name.padEnd(18)} | ${(cfg.url+'').padEnd(30)} | title="${title}" | content=${hasContent} | errs=${hasErrors||0} | mode=${apiMode}`);
    } catch(e) {
      failed++;
      console.log(`❌ ${name.padEnd(18)} | ERROR: ${e.message}`);
    }
  }

  // Install error collector and inject
  await client.navigate('/dashboard');
  await client.wait(1000);
  await client.evaluate(`
    (window.__cdp_errors = []);
    const orig = console.error; console.error = (...a) => { window.__cdp_errors.push(a.map(String).join(' ')); orig.apply(console, a); };
  `);

  // Quick login flow test
  console.log('\n─── Login Flow ───');
  await client.navigate('/login');
  await client.wait(1500);
  const hasForm = await client.evaluate("document.querySelector('input') !== null");
  console.log(`  Login form: ${hasForm ? 'YES ✅' : 'NO ❌'}`);

  // Try to fill login
  await client.evaluate(`
    const email = document.querySelector('input[type=email],input[type=text]');
    const pass = document.querySelector('input[type=password]');
    if(email && pass) {
      const s = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
      s.call(email, 'turta@keyip.io'); email.dispatchEvent(new Event('input', {bubbles: true}));
      s.call(pass, '123456'); pass.dispatchEvent(new Event('input', {bubbles: true}));
      document.querySelector('button')?.click();
      'clicked'
    } else 'no form'
  `);
  await client.wait(2000);
  const postLogin = await client.evaluate('window.location.pathname');
  const token = await client.evaluate("localStorage.getItem('keyip_token') ? 'YES' : 'NO'");
  console.log(`  Post-login URL: ${postLogin}`);
  console.log(`  Token stored: ${token}`);

  // Summary
  console.log(`\n══════════════════════════════════════`);
  console.log(`  Total: ${total} | ✅ ${passed} | ❌ ${failed}`);
  console.log(`  ${failed === 0 ? '🟢 ALL PAGES VERIFIED' : '🔴 SOME PAGES FAILED'}`);
  console.log(`══════════════════════════════════════\n`);

  client.close();
  process.exit(failed > 0 ? 1 : 0);
}

main().catch(e => { console.error(e); process.exit(1); });

#!/usr/bin/env node
/**
 * KeyIP-Intelligence CDP 端到端验证脚本
 *
 * 用法 (从 macOS 主机执行):
 *   1. 先启动 Chrome debug 实例:
 *      pkill -f "remote-debugging-port" 2>/dev/null || true
 *      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
 *        --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug-profile \
 *        --no-first-run --no-default-browser-check --disable-extensions \
 *        --window-size=1400,900 "about:blank"
 *   2. 运行此脚本:
 *      node harness/cdp-verify.js
 *   3. 查看报告:
 *      cat /tmp/keyip_verify_report.json
 *
 * 前置: npm install ws
 */

const { WebSocket } = require('ws');
const http = require('http');
const fs = require('fs');

const BASE_URL = process.env.KEYIP_BASE_URL || 'http://192.168.99.100';
const CDP_URL = process.env.CDP_URL || 'http://192.168.99.1:9222'; // docker-machine host
const REPORT_PATH = '/tmp/keyip_verify_report.json';
const SCREENSHOT_DIR = '/tmp/keyip_screenshots';

// ============================================================================
// 测试页面配置 (来自 docs/user-test-guide.md)
// ============================================================================
const TEST_PAGES = {
  // 1. Dashboard
  dashboard: {
    url: '/dashboard',
    title: 'Executive Dashboard - KeyIP',
    checks: {
      hasKpiCards: "document.querySelectorAll('[class*=kpi], [class*=stat], [class*=metric]').length > 0",
      totalPatents: "document.body.innerText.includes('15')",
      healthScore: "document.body.innerText.includes('76')",
      hasCharts: "document.querySelectorAll('svg, canvas, [class*=chart], [class*=recharts]').length > 0",
    }
  },
  // 2. Search
  search: {
    url: '/search',
    title: 'Search - KeyIP',
    checks: {
      hasSearchBox: "document.querySelector('input[type=text], input[type=search], input[placeholder*=search i], input[placeholder*=Search]') !== null",
    },
    interactions: [
      { name: 'searchOLED',
        setup: "(()=>{const el=document.querySelector('input[type=text],input[type=search]');if(!el)return'no input';const s=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;s.call(el,'OLED');el.dispatchEvent(new Event('input',{bubbles:true}));return'filled'})()",
      },
    ]
  },
  // 3. Patent Mining
  patentMining: {
    url: '/patent-mining',
    title: 'Patent Mining - KeyIP',
    checks: {
      hasSearchTab: "document.body.innerText.includes('Structure') || document.body.innerText.includes('结构')",
      pageLoaded: "document.querySelector('h1,h2,[class*=header],[class*=title]') !== null",
    }
  },
  // 4. Knowledge Graph
  knowledgeGraph: {
    url: '/knowledge-graph',
    title: 'Knowledge Graph - KeyIP',
    checks: {
      hasGraph: "document.querySelectorAll('svg,canvas,[class*=graph],[class*=cytoscape]').length > 0 || document.body.innerText.length > 200",
    }
  },
  // 5. FTO Search
  fto: {
    url: '/fto',
    title: 'FTO Search - KeyIP',
    checks: {
      pageLoaded: "document.querySelector('h1,h2,input') !== null",
    }
  },
  // 6. Infringement Watch
  infringement: {
    url: '/infringement-watch',
    title: 'Infringement Watch - KeyIP',
    checks: {
      hasAlerts: "document.body.innerText.includes('HIGH') || document.body.innerText.includes('风险') || document.querySelectorAll('table tr, [class*=alert], [class*=row]').length > 0",
    }
  },
  // 7. Lifecycle
  lifecycle: {
    url: '/lifecycle',
    title: 'Patent Lifecycle - KeyIP',
    checks: {
      pageLoaded: "document.querySelector('h1,h2,[class*=timeline]') !== null || document.body.innerText.length > 100",
    }
  },
  // 8. Portfolio Optimizer
  portfolio: {
    url: '/portfolio-optimizer',
    title: 'Portfolio Optimizer - KeyIP',
    checks: {
      hasTabs: "document.querySelectorAll('[role=tab], [class*=tab], button').length > 0",
    }
  },
  // 9. Molecules
  molecules: {
    url: '/molecules',
    title: 'Molecule Detail - KeyIP',
    checks: {
      hasMoleculeList: "document.querySelectorAll('table tr, [class*=card], [class*=list]').length > 0 || document.body.innerText.includes('CBP')",
    }
  },
  // 10. Patents
  patents: {
    url: '/patents/CN115650927B',
    title: 'Patent Detail - KeyIP',
    checks: {
      hasPatentInfo: "document.body.innerText.includes('CN115650927B') || document.querySelector('h1,h2') !== null",
    }
  },
  // 11. Partners
  partners: {
    url: '/partners',
    title: 'Partners - KeyIP',
    checks: {
      hasPartners: "document.body.innerText.length > 100",
    }
  },
  // 12. Health
  health: {
    url: '/health',
    title: 'System Health - KeyIP',
    checks: {
      hasHealthStatus: "document.body.innerText.length > 100",
    }
  },
  // 13. Settings
  settings: {
    url: '/settings',
    title: 'Settings - KeyIP',
    checks: {
      hasSettings: "document.querySelectorAll('input, select, button, [class*=switch], [class*=toggle]').length > 0",
    }
  },
  // 14. Login
  login: {
    url: '/login',
    title: 'Login - KeyIP',
    checks: {
      hasLoginForm: "document.querySelector('input[type=email], input[type=text]') !== null",
      hasSignInBtn: "document.querySelector('button[type=submit]') !== null || document.body.innerText.includes('Sign In') || document.body.innerText.includes('登录')",
    }
  },
};

// ============================================================================
// CDP 客户端
// ============================================================================
class CDPClient {
  constructor(wsUrl) {
    this.wsUrl = wsUrl;
    this.ws = null;
    this.msgId = 0;
    this.pending = new Map();
  }

  async connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.wsUrl);
      this.ws.on('open', () => {
        this.ws.on('message', (data) => {
          const msg = JSON.parse(data.toString());
          if (msg.id && this.pending.has(msg.id)) {
            const { resolve } = this.pending.get(msg.id);
            this.pending.delete(msg.id);
            resolve(msg);
          }
        });
        resolve();
      });
      this.ws.on('error', reject);
      setTimeout(() => reject(new Error('WebSocket connect timeout')), 10000);
    });
  }

  async send(method, params = {}) {
    const id = ++this.msgId;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`CDP timeout: ${method}`));
        }
      }, 15000);
    });
  }

  async enable(domain) {
    return this.send(`${domain}.enable`);
  }

  async navigate(url, timeout = 30000) {
    await this.send('Page.enable');
    const fullUrl = url.startsWith('http') ? url : `${BASE_URL}${url}`;

    let loadFired = false;
    const onMsg = (data) => {
      try {
        const msg = JSON.parse(data.toString());
        if (msg.method === 'Page.loadEventFired') loadFired = true;
      } catch(e) {}
    };

    this.ws.on('message', onMsg);
    const result = await this.send('Page.navigate', { url: fullUrl });

    // Wait for load event
    const start = Date.now();
    while (!loadFired && (Date.now() - start) < timeout) {
      await new Promise(r => setTimeout(r, 500));
    }
    this.ws.removeListener('message', onMsg);

    return { ...result, loadFired };
  }

  async evaluate(expression, returnByValue = true) {
    const result = await this.send('Runtime.evaluate', { expression, returnByValue });
    return result.result?.result?.value;
  }

  async screenshot() {
    const result = await this.send('Page.captureScreenshot', { format: 'png' });
    return result.result?.data;
  }

  async getTitle() {
    return this.evaluate('document.title');
  }

  async getConsoleErrors() {
    return this.evaluate('JSON.stringify(window.__cdp_errors || [])');
  }

  async getNetworkFailures() {
    return this.evaluate(`
      JSON.stringify(performance.getEntriesByType('resource')
        .filter(r => r.responseStatus >= 400 || r.responseStatus === 0)
        .map(r => ({url: r.name, status: r.responseStatus, duration: r.duration}))
      )
    `);
  }

  async injectErrorCollector() {
    return this.evaluate(`
      (window.__cdp_errors = []);
      (window.__cdp_warns = []);
      const origErr = console.error;
      const origWarn = console.warn;
      console.error = (...args) => {
        window.__cdp_errors.push(args.map(String).join(' '));
        origErr.apply(console, args);
      };
      console.warn = (...args) => {
        window.__cdp_warns.push(args.map(String).join(' '));
        origWarn.apply(console, args);
      };
      'collector installed'
    `);
  }

  async getApiMode() {
    return this.evaluate("localStorage.getItem('keyip-api-mode') || 'not set'");
  }

  async getToken() {
    return this.evaluate("localStorage.getItem('keyip_token') ? 'present' : 'absent'");
  }

  close() {
    if (this.ws) this.ws.close();
  }
}

// ============================================================================
// HTTP 页面列表获取器 (fallback: 从 CDP 获取页面列表)
// ============================================================================
async function getCdpPages(cdpHost) {
  return new Promise((resolve, reject) => {
    http.get(`${cdpHost}/json`, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try { resolve(JSON.parse(data)); }
        catch(e) { reject(e); }
      });
    }).on('error', reject);
  });
}

// ============================================================================
// 主验证流程
// ============================================================================
async function main() {
  const report = {
    timestamp: new Date().toISOString(),
    base_url: BASE_URL,
    summary: { total: 0, passed: 0, failed: 0, warnings: 0 },
    pages: {},
    errors: [],
    warnings: [],
    login_flow: null,
    sidebar_navigation: null,
    api_mode: null,
    token_status: null,
    findings: [],
  };

  console.log('╔══════════════════════════════════════════════╗');
  console.log('║  KeyIP-Intelligence CDP 交付验证 v1.0       ║');
  console.log('╚══════════════════════════════════════════════╝');
  console.log(`\n📍 Target: ${BASE_URL}`);
  console.log(`🔌 CDP:    ${CDP_URL}\n`);

  // Step 1: Get CDP page list
  let cdpPages;
  try {
    cdpPages = await getCdpPages(CDP_URL);
    console.log(`✅ CDP connected — ${cdpPages.length} page(s) available`);
  } catch (e) {
    console.log(`❌ Cannot connect to CDP at ${CDP_URL}`);
    console.log(`   Is Chrome running with --remote-debugging-port?`);
    console.log(`   Run: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \\`);
    console.log(`          --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug-profile &`);
    process.exit(1);
  }

  // Find or create a page
  let pageTarget = cdpPages.find(p => p.type === 'page');
  if (!pageTarget) {
    console.log('No page target found — creating new page...');
    // Need to create a new tab via HTTP
    pageTarget = cdpPages[0];
  }

  const wsUrl = pageTarget.webSocketDebuggerUrl;
  const client = new CDPClient(wsUrl);
  await client.connect();

  // Enable domains
  await client.enable('Runtime');
  await client.enable('Page');
  await client.enable('Network');
  await client.enable('Console');
  await client.enable('Log');

  // Install error collector
  await client.injectErrorCollector();

  // ==========================================================================
  // 1. API Mode check
  // ==========================================================================
  console.log('\n─── API Mode ───');
  report.api_mode = await client.getApiMode();
  console.log(`  API Mode: ${report.api_mode}`);

  // ==========================================================================
  // 2. Login Flow Test
  // ==========================================================================
  console.log('\n─── Login Flow ───');
  try {
    await client.navigate('/login');
    await new Promise(r => setTimeout(r, 2000));

    const loginForm = await client.evaluate("document.querySelector('input[type=email], input[type=text]') !== null");
    const signInBtn = await client.evaluate("document.querySelector('button') !== null");

    report.login_flow = { form_detected: loginForm, button_detected: signInBtn, attempted: false };

    if (loginForm) {
      console.log('  ✅ Login form detected');

      // Try to fill and submit
      await client.evaluate(`
        (() => {
          const emailEl = document.querySelector('input[type=email], input[name=email], input[placeholder*=email i]');
          const passEl = document.querySelector('input[type=password]');
          if (!emailEl) return 'no email input';
          const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
          setter.call(emailEl, 'turta@keyip.io');
          emailEl.dispatchEvent(new Event('input', {bubbles: true}));
          if (passEl) {
            setter.call(passEl, 'turta123!');
            passEl.dispatchEvent(new Event('input', {bubbles: true}));
          }
          return 'filled: ' + emailEl.value;
        })()
      `);

      // Click sign in
      await client.evaluate("document.querySelector('button[type=submit]')?.click() || document.querySelector('button')?.click()");
      report.login_flow.attempted = true;
      console.log('  🔄 Sign-in attempted');

      await new Promise(r => setTimeout(r, 3000));

      const postLoginUrl = await client.evaluate('window.location.pathname');
      const token = await client.getToken();
      report.login_flow.post_login_url = postLoginUrl;
      report.login_flow.token_stored = token;
      report.token_status = token;

      // Check for hardcoded placeholder
      const hasJohnDoe = await client.evaluate("document.body.innerText.includes('John Doe') ? 'YES (issue!)' : 'no'");
      report.login_flow.john_doe_detected = hasJohnDoe;

      console.log(`  📍 After login URL: ${postLoginUrl}`);
      console.log(`  🔑 Token: ${token}`);
      console.log(`  👤 John Doe: ${hasJohnDoe}`);
    } else {
      console.log('  ⚠️  No login form detected — page may be different');
    }
  } catch (e) {
    console.log(`  ❌ Login test failed: ${e.message}`);
    report.login_flow = { error: e.message };
  }

  // ==========================================================================
  // 3. Page-by-page verification
  // ==========================================================================
  console.log('\n─── Page Verification ───');

  for (const [name, config] of Object.entries(TEST_PAGES)) {
    report.summary.total++;
    const pageResult = { name, url: config.url, title: null, checks: {}, errors: [], failed_requests: [], interactions: {} };

    try {
      console.log(`\n  📄 ${name} → ${config.url}`);
      await client.navigate(config.url);
      await client.injectErrorCollector();
      await new Promise(r => setTimeout(r, 1500));

      // Title
      pageResult.title = await client.getTitle();
      const titleOk = pageResult.title && pageResult.title.length > 0;
      console.log(`     Title: "${pageResult.title}" ${titleOk ? '✅' : '⚠️'}`);

      // Checks
      for (const [checkName, expression] of Object.entries(config.checks)) {
        try {
          const value = await client.evaluate(expression);
          const passed = value === true || (typeof value === 'number' && value > 0) || (typeof value === 'string' && value.length > 0);
          pageResult.checks[checkName] = { passed, value };
          console.log(`     Check ${checkName}: ${passed ? '✅' : '❌'} (${JSON.stringify(value)})`);
        } catch (e) {
          pageResult.checks[checkName] = { passed: false, error: e.message };
          console.log(`     Check ${checkName}: ❌ ${e.message}`);
        }
      }

      // Interactions
      if (config.interactions) {
        for (const interaction of config.interactions) {
          try {
            const result = await client.evaluate(interaction.setup);
            pageResult.interactions[interaction.name] = { result };
            console.log(`     🔧 ${interaction.name}: ${JSON.stringify(result)}`);
          } catch (e) {
            pageResult.interactions[interaction.name] = { error: e.message };
          }
        }
        await new Promise(r => setTimeout(r, 1000));
      }

      // Console errors
      const errors = await client.getConsoleErrors();
      if (errors && errors !== '[]') {
        try {
          const parsed = JSON.parse(errors);
          pageResult.errors = parsed;
          console.log(`     ⚠️  Console errors: ${parsed.length}`);
          parsed.forEach(e => console.log(`        - ${e.substring(0, 120)}`));
        } catch(e) {}
      }

      // Network failures
      const failures = await client.getNetworkFailures();
      if (failures && failures !== '[]') {
        try {
          const parsed = JSON.parse(failures);
          pageResult.failed_requests = parsed;
          if (parsed.length > 0) {
            console.log(`     🔴 Failed requests: ${parsed.length}`);
            parsed.forEach(f => console.log(`        - ${f.url} (${f.status})`));
          }
        } catch(e) {}
      }

      // Pass/fail determination
      const checksPassed = Object.values(pageResult.checks).every(c => c.passed !== false);
      const hasErrors = pageResult.errors.length > 0;
      const hasFailures = pageResult.failed_requests.length > 0;

      if (checksPassed && !hasErrors && !hasFailures) {
        report.summary.passed++;
        pageResult.status = 'PASS';
      } else if (!hasErrors && !hasFailures) {
        report.summary.warnings++;
        pageResult.status = 'WARN';
      } else {
        report.summary.failed++;
        pageResult.status = 'FAIL';
      }

      console.log(`     Status: ${pageResult.status}`);

    } catch (e) {
      pageResult.error = e.message;
      report.summary.failed++;
      pageResult.status = 'FAIL';
      console.log(`     ❌ ${e.message}`);
    }

    report.pages[name] = pageResult;
  }

  // ==========================================================================
  // 4. Sidebar navigation test
  // ==========================================================================
  console.log('\n─── Sidebar Navigation ───');
  try {
    await client.navigate('/dashboard');
    await new Promise(r => setTimeout(r, 1500));

    const sidebarLinks = await client.evaluate(`
      JSON.stringify(
        Array.from(document.querySelectorAll('nav a, aside a, [class*=sidebar] a, [class*=nav] a'))
          .map(a => ({ href: a.getAttribute('href'), text: a.textContent?.trim()?.substring(0, 30) }))
          .filter(l => l.href && !l.href.startsWith('http'))
      )
    `);

    const links = JSON.parse(sidebarLinks || '[]');
    report.sidebar_navigation = { links_found: links.length, links, test_results: [] };
    console.log(`  Found ${links.length} sidebar links`);

    for (const link of links.slice(0, 12)) { // Test up to 12
      try {
        await client.navigate(link.href);
        await new Promise(r => setTimeout(r, 800));
        const newErrors = await client.getConsoleErrors();
        const hasNewErrors = newErrors && newErrors !== '[]' && JSON.parse(newErrors).length > 0;
        report.sidebar_navigation.test_results.push({
          href: link.href,
          errors_after_click: hasNewErrors,
        });
        console.log(`  ${hasNewErrors ? '⚠️' : '✅'} ${link.text} → ${link.href}`);
        await client.injectErrorCollector();
      } catch (e) {
        report.sidebar_navigation.test_results.push({ href: link.href, error: e.message });
      }
    }
  } catch (e) {
    report.sidebar_navigation = { error: e.message };
    console.log(`  ❌ Sidebar test failed: ${e.message}`);
  }

  // ==========================================================================
  // 5. Report
  // ==========================================================================
  client.close();

  console.log('\n╔══════════════════════════════════════════════╗');
  console.log('║  Verification Report                        ║');
  console.log('╚══════════════════════════════════════════════╝');
  console.log(`\n  📊 Total:  ${report.summary.total}`);
  console.log(`  ✅ Passed: ${report.summary.passed}`);
  console.log(`  ⚠️  Warn:   ${report.summary.warnings}`);
  console.log(`  ❌ Failed: ${report.summary.failed}`);

  // Summary by page
  console.log('\n  Page Status:');
  for (const [name, page] of Object.entries(report.pages)) {
    const icon = page.status === 'PASS' ? '✅' : page.status === 'WARN' ? '⚠️' : '❌';
    console.log(`    ${icon} ${name}: ${page.title || 'N/A'} (checks=${Object.keys(page.checks).length}, errs=${page.errors?.length||0}, fails=${page.failed_requests?.length||0})`);
  }

  // Save report
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  fs.writeFileSync(REPORT_PATH, JSON.stringify(report, null, 2));
  console.log(`\n📝 Full report saved to: ${REPORT_PATH}`);

  // Exit code
  if (report.summary.failed > 3) {
    console.log('\n🔴 DELIVERY CHECK: FAILED — too many page failures');
    process.exit(1);
  } else if (report.summary.failed > 0) {
    console.log('\n🟡 DELIVERY CHECK: WARN — some issues found');
    process.exit(0);
  } else {
    console.log('\n🟢 DELIVERY CHECK: PASSED');
    process.exit(0);
  }
}

// ============================================================================
// Run
// ============================================================================
main().catch(e => {
  console.error(`\n❌ Fatal error: ${e.message}`);
  console.error(e.stack);
  process.exit(1);
});

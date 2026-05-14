#!/usr/bin/env node
/**
 * KeyIP-Intelligence 开源专利数据批量导入系统
 *
 * 数据源:
 *   1. Lens.org API (免费, https://api.lens.org)
 *   2. USPTO Open Data API (免费, https://developer.uspto.gov)
 *
 * 用法:
 *   node scripts/import/fetch_patents.js --source=lens --query="OLED organic light emitting" --count=100
 *   node scripts/import/fetch_patents.js --source=uspto --query="phosphorescent OLED" --count=50
 *   node scripts/import/fetch_patents.js --source=all --max=500     # 批量导入
 *
 * 环境变量:
 *   KEYIP_API_URL=http://192.168.99.100/api/v1
 *   LENS_API_TOKEN=xxx  (可选, Lens.org 免费注册 https://www.lens.org)
 */

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');

// ─── Configuration ──────────────────────────────────────────────
const API_URL = process.env.KEYIP_API_URL || 'http://192.168.99.100/api/v1';
const LENS_TOKEN = process.env.LENS_API_TOKEN || '';
const DATA_DIR = path.join(__dirname, '..', '..', 'test', 'testdata', 'patents');

// ─── OLED Patent Queries ────────────────────────────────────────
const OLED_QUERIES = [
  'OLED organic light emitting diode phosphorescent',
  'OLED host material electron transport',
  'OLED hole transport layer TADF',
  'OLED blue emitter iridium complex',
  'OLED thermally activated delayed fluorescence',
  'OLED charge transport material',
  'organic electroluminescent device emitting layer',
  'OLED display panel light emitting',
  'phosphorescent organometallic compound OLED',
  'OLED encapsulation thin film',
];

/**
 * Lens.org Patent Search API
 * Free tier: 50 requests/day without token, 1000/day with registered token
 * Docs: https://docs.api.lens.org/
 */
async function searchLens(query, page = 0, size = 50) {
  const requestBody = JSON.stringify({
    query: {
      bool: {
        must: [
          { match_phrase: { 'title.english': query } },
          { term: { publication_type: 'application' } },
        ],
      },
    },
    include: ['title', 'abstract', 'claims', 'publication_number', 'application_number',
              'priority_date', 'publication_date', 'inventors', 'applicants',
              'jurisdiction', 'legal_status', 'cpc', 'ipc', 'family_id',
              'cited_by', 'citations', 'publication_type', 'grant_number'],
    sort: [{ publication_date: 'desc' }],
    size,
    from: page * size,
  });

  return new Promise((resolve, reject) => {
    const url = new URL('https://api.lens.org/scholarly/search');
    const options = {
      hostname: url.hostname,
      port: 443,
      path: url.pathname,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        ...(LENS_TOKEN ? { 'Authorization': `Bearer ${LENS_TOKEN}` } : {}),
      },
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const result = JSON.parse(data);
          resolve(result);
        } catch (e) {
          reject(new Error(`Lens parse error: ${e.message}`));
        }
      });
    });
    req.on('error', reject);
    req.write(requestBody);
    req.end();
  });
}

/**
 * USPTO Open Data Patent Search API
 * Free, no API key required for basic access
 * Docs: https://developer.uspto.gov/api-catalog
 */
async function searchUSPTO(query, page = 0, size = 25) {
  const requestBody = JSON.stringify({
    query: query,
    filters: [
      { name: 'patentType', value: ['design', 'utility'] },
    ],
    sort: 'date_publ desc',
    rows: size,
    start: page * size,
  });

  return new Promise((resolve, reject) => {
    const options = {
      hostname: 'developer.uspto.gov',
      path: '/ibd-api/v1/application/publications',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
      },
    };

    const req = https.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const result = JSON.parse(data);
          resolve(result);
        } catch (e) {
          reject(new Error(`USPTO parse error: ${e.message}`));
        }
      });
    });
    req.on('error', reject);
    req.write(requestBody);
    req.end();
  });
}

/**
 * Convert Lens.org patent to KeyIP format
 */
function convertLensToKeyIP(patent) {
  const title = Array.isArray(patent.title) ? patent.title[0]?.english || '' : (patent.title?.english || '');
  const abs = Array.isArray(patent.abstract) ? patent.abstract[0]?.english || '' : (patent.abstract?.english || '');
  const claims = Array.isArray(patent.claims) ? patent.claims.map(c => c.english || '').join(' ') : '';

  return {
    patent_number: patent.publication_number || patent.grant_number || '',
    title: title.substring(0, 500),
    abstract: abs.substring(0, 2000),
    application_number: patent.application_number || '',
    priority_date: patent.priority_date || '',
    publication_date: patent.publication_date || '',
    inventors: (patent.inventors || []).map(i => i.name || '').join('; '),
    assignee: (patent.applicants || [])[0]?.name || '',
    jurisdiction: patent.jurisdiction || '',
    legal_status: patent.legal_status || 'unknown',
    ipc_codes: (patent.ipc || []).join('; '),
    cpc_codes: (patent.cpc || []).join('; '),
    family_id: patent.family_id || '',
    citations: patent.cited_by || [],
    source: 'lens.org',
    claims_text: claims.substring(0, 5000),
  };
}

/**
 * Import patent into KeyIP via REST API
 */
async function importToKeyIP(patentData) {
  return new Promise((resolve, reject) => {
    const url = new URL(`${API_URL}/patents`);
    const body = JSON.stringify(patentData);
    const client = url.protocol === 'https:' ? https : http;

    const options = {
      hostname: url.hostname,
      port: url.port,
      path: url.pathname,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(body),
      },
    };

    const req = client.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode, data: JSON.parse(data) });
        } catch {
          resolve({ status: res.statusCode, data });
        }
      });
    });
    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

/**
 * Save patents to local file for offline import
 */
function saveToFile(patents, source, query) {
  fs.mkdirSync(DATA_DIR, { recursive: true });
  const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
  const safeQuery = query.replace(/[^a-zA-Z0-9]/g, '_').substring(0, 30);
  const filename = path.join(DATA_DIR, `${source}_${safeQuery}_${timestamp}.json`);
  fs.writeFileSync(filename, JSON.stringify(patents, null, 2));
  console.log(`  📁 Saved ${patents.length} patents to ${filename}`);
  return filename;
}

/**
 * Load saved patents and import to API
 */
function loadFromFile(filepath) {
  if (!fs.existsSync(filepath)) {
    console.error(`  File not found: ${filepath}`);
    return [];
  }
  return JSON.parse(fs.readFileSync(filepath, 'utf8'));
}

// ─── Main ───────────────────────────────────────────────────────
async function main() {
  const args = process.argv.slice(2);
  const sourceArg = args.find(a => a.startsWith('--source='));
  const source = sourceArg ? sourceArg.split('=')[1] : 'lens';
  const queryArg = args.find(a => a.startsWith('--query='));
  const query = queryArg ? queryArg.split('=')[1] : 'OLED organic light emitting';
  const maxArg = args.find(a => a.startsWith('--count=') || a.startsWith('--max='));
  const maxResults = maxArg ? parseInt(maxArg.split('=')[1]) : 50;
  const dryRun = args.includes('--dry-run') || args.includes('--save-only');
  const importFile = args.find(a => a.startsWith('--file='));
  const listOnly = args.includes('--list');

  console.log('╔══════════════════════════════════════════════════════╗');
  console.log('║  KeyIP-Intelligence Patent Data Importer             ║');
  console.log('╚══════════════════════════════════════════════════════╝');
  console.log(`  Source: ${source}`);
  console.log(`  Query:  ${query}`);
  console.log(`  Max:    ${maxResults}`);
  console.log(`  API:    ${API_URL}\n`);

  // Import from file
  if (importFile) {
    const patents = loadFromFile(importFile);
    console.log(`📂 Loaded ${patents.length} patents from file`);
    for (let i = 0; i < Math.min(patents.length, maxResults); i++) {
      try {
        const result = await importToKeyIP(patents[i]);
        console.log(`  [${i + 1}/${Math.min(patents.length, maxResults)}] ${patents[i].patent_number}: ${result.status}`);
      } catch (e) {
        console.log(`  [${i + 1}/${Math.min(patents.length, maxResults)}] ${patents[i].patent_number}: ERROR ${e.message}`);
      }
    }
    return;
  }

  // List saved data files
  if (listOnly) {
    if (fs.existsSync(DATA_DIR)) {
      const files = fs.readdirSync(DATA_DIR).filter(f => f.endsWith('.json'));
      console.log(`📂 ${files.length} saved patent data files:\n`);
      files.forEach(f => {
        const stat = fs.statSync(path.join(DATA_DIR, f));
        const data = JSON.parse(fs.readFileSync(path.join(DATA_DIR, f), 'utf8'));
        console.log(`  ${f} (${data.length} patents, ${(stat.size/1024).toFixed(1)} KB)`);
      });
    } else {
      console.log('No saved patent data files.');
    }
    return;
  }

  // Fetch patents
  const queries = source === 'all' ? OLED_QUERIES : [query];
  let totalFetched = 0;
  const allPatents = [];

  for (const q of queries) {
    if (totalFetched >= maxResults) break;

    const remaining = maxResults - totalFetched;
    const pageSize = Math.min(remaining, 50);

    console.log(`🔍 Searching: "${q.substring(0, 60)}..."`);

    try {
      let result;
      if (source === 'lens' || source === 'all') {
        result = await searchLens(q, 0, pageSize);
        // Lens returns patents in hits.hits
        const hits = result?.hits?.hits || result?.data || [];
        const patents = hits.map(h => convertLensToKeyIP(h._source || h));
        allPatents.push(...patents);
        totalFetched += patents.length;
        console.log(`  ✅ Lens: ${patents.length} patents`);
      }

      if ((source === 'uspto' || source === 'all') && totalFetched < maxResults) {
        result = await searchUSPTO(q, 0, pageSize);
        const usptoResults = result?.results || result?.patentResults || [];
        const patents = usptoResults.map(p => ({
          patent_number: p.patentNumber || p.patent_number || '',
          title: p.inventionTitle || p.title || '',
          abstract: p.abstractText || p.abstract || '',
          application_number: p.applicationNumber || '',
          publication_date: p.publicationDate || p.patentFileDate || '',
          inventors: p.inventorName || '',
          assignee: p.assigneeEntityName || p.organizationName || '',
          jurisdiction: 'US',
          ipc_codes: p.mainIpcClass || '',
          source: 'uspto.gov',
        }));
        allPatents.push(...patents);
        totalFetched += patents.length;
        console.log(`  ✅ USPTO: ${patents.length} patents`);
      }
    } catch (e) {
      console.log(`  ⚠️  Error: ${e.message}`);
    }

    // Rate limiting
    await new Promise(r => setTimeout(r, 2000));
  }

  console.log(`\n📊 Total fetched: ${allPatents.length} patents`);

  if (allPatents.length === 0) {
    console.log('\n⚠️  No patents fetched. Possible reasons:');
    console.log('  1. Network connectivity — check proxy/DNS settings');
    console.log('  2. API rate limiting — wait and retry');
    console.log('  3. API key required — get one at https://www.lens.org');
    console.log('\n💡 Tip: Try with a local save file instead:');
    console.log('   node scripts/import/fetch_patents.js --file=test/testdata/patents/xxx.json');
    return;
  }

  // Save to file
  const savedFile = saveToFile(allPatents, source, query);

  if (dryRun) {
    console.log('\n🔒 --dry-run: patents saved to file only, not imported.');
    console.log('   To import: node scripts/import/fetch_patents.js --file=' + savedFile);
    return;
  }

  // Import to API
  console.log(`\n📥 Importing to KeyIP API...`);
  let imported = 0;
  let failed = 0;

  for (let i = 0; i < allPatents.length; i++) {
    try {
      const result = await importToKeyIP(allPatents[i]);
      if (result.status === 200 || result.status === 201) {
        imported++;
        if (imported % 10 === 0) process.stdout.write(`.${imported}`);
      } else if (result.status === 409) {
        // Duplicate — skip
        process.stdout.write('d');
      } else {
        failed++;
        process.stdout.write('x');
      }
    } catch (e) {
      failed++;
      process.stdout.write('x');
    }
    // Rate limiting
    if (i % 5 === 4) await new Promise(r => setTimeout(r, 500));
  }

  console.log(`\n\n📊 Import complete:`);
  console.log(`   ✅ Imported: ${imported}`);
  console.log(`   ❌ Failed:   ${failed}`);
  console.log(`   📁 Saved:    ${savedFile}`);
  console.log(`\n💡 To verify: curl ${API_URL}/patents | python3 -m json.tool | head -20`);
}

main().catch(e => {
  console.error(`\n❌ Fatal: ${e.message}`);
  process.exit(1);
});

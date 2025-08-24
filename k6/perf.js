import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Counter, Rate } from 'k6/metrics';

/* =========================
   Usage (contoh):
   - Smoke:
     k6 run k6/perf.js
   - Constant 500 RPS selama 2 menit:
     k6 run -e URL=https://backend-api.oksasatya.dev/api/ip -e RATE=500 -e DURATION=120s k6/perf.js
   - Target 5k RPS selama 200s:
     k6 run -e URL=https://backend-api.oksasatya.dev/api/ip -e RATE=5000 -e DURATION=200s k6/perf.js
   - Ramping 1k -> 10k RPS, naik 60 detik lalu tahan 120 detik:
     k6 run -e SCENARIO=ramping -e START_RATE=1000 -e END_RATE=10000 -e STAGES=60 -e DURATION=120s k6/perf.js
   ========================= */

const DEFAULT_URL = __ENV.URL || 'https://backend-api.oksasatya.dev/api/ip';
const METHOD = (__ENV.METHOD || 'GET').toUpperCase();
const BODY = __ENV.BODY || '';
const CONTENT_TYPE = __ENV.CONTENT_TYPE || 'application/json';
const HEADERS = { 'Content-Type': CONTENT_TYPE, ...(parseKV(__ENV.HEADERS) || {}) };

const RATE = toNum(__ENV.RATE, 200);               // req/s untuk constant-arrival
const DURATION = __ENV.DURATION || '30s';          // durasi test
const PRE_ALLOC_VUS = toNum(__ENV.VUS, 200);       // VU buffer awal
const MAX_VUS = toNum(__ENV.MAX_VUS, PRE_ALLOC_VUS * 5);

const START_RATE = toNum(__ENV.START_RATE, 100);   // awal ramp
const END_RATE = toNum(__ENV.END_RATE, 1000);      // akhir ramp
const STAGES_SEC = toNum(__ENV.STAGES, 60);        // waktu naik (detik)

// ===== Custom Metrics =====
const ttfb = new Trend('ttfb');              // time-to-first-byte
const http2xx = new Counter('http_2xx');
const http3xx = new Counter('http_3xx');
const http4xx = new Counter('http_4xx');
const http429 = new Counter('http_429');
const http5xx = new Counter('http_5xx');
const req_ok = new Rate('req_ok');

// ===== K6 Options =====
export const options = {
    scenarios: selectScenario(),
    discardResponseBodies: toBool(__ENV.DISCARD_BODY, true), // hemat bandwidth
    insecureSkipTLSVerify: toBool(__ENV.INSECURE, false),
    batchPerHost: toNum(__ENV.BATCH_PER_HOST, 0),            // 0 = unlimited
    gracefulStop: __ENV.GRACEFUL_STOP || '0s',               // stop tegas di akhir
    thresholds: {
        http_req_failed: ['rate<0.01'],         // < 1% gagal
        req_ok: ['rate>0.99'],                  // > 99% sukses
        http_req_duration: [
            'p(95)<1000',                         // p95 < 1s (awal realistis)
            'p(99)<3000',                         // p99 < 3s
        ],
        ttfb: ['p(95)<800'],                    // TTFB p95 < 800ms
        http_5xx: ['count==0'],                 // idealnya tanpa 5xx
    },
};

function selectScenario() {
    const type = (__ENV.SCENARIO || 'constant').toLowerCase();

    if (type === 'ramping') {
        return {
            ramping_arrival: {
                executor: 'ramping-arrival-rate',
                startRate: START_RATE,
                timeUnit: '1s',
                preAllocatedVUs: PRE_ALLOC_VUS,
                maxVUs: MAX_VUS,
                stages: [
                    { target: END_RATE, duration: `${STAGES_SEC}s` }, // naik
                    { target: END_RATE, duration: DURATION },         // tahan
                ],
            },
        };
    }

    // default: constant-arrival-rate
    return {
        constant_arrival: {
            executor: 'constant-arrival-rate',
            rate: RATE,
            timeUnit: '1s',
            duration: DURATION,
            preAllocatedVUs: PRE_ALLOC_VUS,
            maxVUs: MAX_VUS,
        },
    };
}

// ===== Main VU Function =====
export default function () {
    const url = withCacheBuster(DEFAULT_URL);
    const params = {
        headers: HEADERS,
        redirects: toNum(__ENV.REDIRECTS, 10),
        timeout: __ENV.TIMEOUT || '30s',
        tags: {
            svc: __ENV.SVC || 'nuvora',
            env: __ENV.ENV || 'prod',
        },
    };

    let res;
    switch (METHOD) {
        case 'POST':
        case 'PUT':
        case 'PATCH':
            res = http.request(METHOD, url, BODY, params);
            break;
        default:
            res = http.get(url, params);
    }

    // ===== Metrics & Checks =====
    ttfb.add(res.timings.waiting);
    req_ok.add(res.status >= 200 && res.status < 400);

    if (res.status >= 200 && res.status < 300) http2xx.add(1);
    else if (res.status >= 300 && res.status < 400) http3xx.add(1);
    else if (res.status === 429) { http429.add(1); http4xx.add(1); }
    else if (res.status >= 400 && res.status < 500) http4xx.add(1);
    else if (res.status >= 500) http5xx.add(1);

    check(res, {
        'status is 2xx/3xx': (r) => r.status >= 200 && r.status < 400,
    });

    const s = toNum(__ENV.SLEEP_MS, 0) / 1000; // smoothing scheduler
    if (s > 0) sleep(s);
}

// ===== Helpers =====
function withCacheBuster(u) {
    if (toBool(__ENV.BUST_CACHE, true) !== true) return u;
    const sep = u.includes('?') ? '&' : '?';
    return `${u}${sep}cb=${Math.random().toString(36).slice(2)}`;
}

function toNum(v, def) {
    const n = Number(v);
    return Number.isFinite(n) && n >= 0 ? n : def;
}

function toBool(v, def) {
    if (v === undefined) return def;
    const s = String(v).toLowerCase();
    if (s === 'true' || s === '1') return true;
    if (s === 'false' || s === '0') return false;
    return def;
}

function parseKV(s) {
    // Parse simple header overrides: "Authorization=Bearer x, X-Req-Id=abc"
    if (!s) return null;
    return String(s)
        .split(',')
        .map((p) => p.trim())
        .filter(Boolean)
        .reduce((acc, pair) => {
            const idx = pair.indexOf('=');
            if (idx > 0) {
                const k = pair.slice(0, idx).trim();
                const v = pair.slice(idx + 1).trim();
                acc[k] = v;
            }
            return acc;
        }, {});
}

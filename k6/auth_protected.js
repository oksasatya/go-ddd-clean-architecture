import http from 'k6/http';
import { check, sleep } from 'k6';

/*
Env vars:
- BASE (default: https://backend-api.oksasatya.dev/api)  <-- dengan /api
- PROTECTED_PATH (default: /profile)  <-- dipanggil ke BASE + PROTECTED_PATH
- REFRESH_TOKEN (opsional)  <-- bootstrap via /api/refresh
- EMAIL, PASSWORD, DEVICE_ID (opsional)  <-- trusted-device login via /api/login
- HEADERS (opsional; mis. "X-Env=perf, X-Req-Id=abc")
- RATE, DURATION, VUS, MAX_VUS, SCENARIO, START_RATE, END_RATE, STAGES, INSECURE, SLEEP_MS
*/

const BASE = (__ENV.BASE || 'https://backend-api.oksasatya.dev/api').replace(/\/+$/, ''); // .../api
const ORIGIN = BASE.replace(/\/api$/, ''); // https://backend-api.oksasatya.dev
const PROTECTED_PATH = __ENV.PROTECTED_PATH || '/profile';

const EMAIL = __ENV.EMAIL || '';
const PASSWORD = __ENV.PASSWORD || '';
const DEVICE_ID = __ENV.DEVICE_ID || '';
const REFRESH_TOKEN = __ENV.REFRESH_TOKEN || '';

const HEADERS = { 'Content-Type': 'application/json', ...(parseKV(__ENV.HEADERS) || {}) };

const RATE = toNum(__ENV.RATE, 200);
const DURATION = __ENV.DURATION || '60s';
const PRE_ALLOC_VUS = toNum(__ENV.VUS, 100);
const MAX_VUS = toNum(__ENV.MAX_VUS, PRE_ALLOC_VUS * 5);
const START_RATE = toNum(__ENV.START_RATE, 100);
const END_RATE = toNum(__ENV.END_RATE, 1000);
const STAGES_SEC = toNum(__ENV.STAGES, 60);

export const options = {
    scenarios: selectScenario(),
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(95)<1000', 'p(99)<3000'],
    },
    insecureSkipTLSVerify: toBool(__ENV.INSECURE, false),
};

const bootstrapped = {};

export default function () {
    const jar = http.cookieJar();

    if (!bootstrapped[__VU]) {
        if (!bootstrapVU(jar)) {
            throw new Error('Bootstrap failed. Provide REFRESH_TOKEN or a pre-trusted DEVICE_ID.');
        }
        bootstrapped[__VU] = true;
    }

    const res = http.get(`${BASE}${PROTECTED_PATH}`, { headers: HEADERS });
    check(res, { 'protected 200': (r) => r.status === 200 });

    const s = toNum(__ENV.SLEEP_MS, 0) / 1000;
    if (s > 0) sleep(s);
}

function bootstrapVU(jar) {
    // A) REFRESH_TOKEN → coba cookie dulu, kalau gagal coba body JSON
    if (REFRESH_TOKEN) {
        // set cookie refresh_token di ORIGIN dengan path "/"
        jar.set(ORIGIN, 'refresh_token', REFRESH_TOKEN, { path: '/', secure: true });

        // 1) Cookie-only
        let r = http.post(`${BASE}/refresh`, null, { headers: HEADERS });
        if (r.status === 200) return true;
        console.error('refresh(cookie) failed:', r.status, safeBody(r.body));

        // 2) Body JSON
        const body = JSON.stringify({ refresh_token: REFRESH_TOKEN });
        r = http.post(`${BASE}/refresh`, body, { headers: HEADERS });
        if (r.status === 200) return true;
        console.error('refresh(body) failed:', r.status, safeBody(r.body));

        return false;
    }

    // B) Trusted device login
    if (DEVICE_ID && EMAIL && PASSWORD) {
        jar.set(ORIGIN, 'device_id', DEVICE_ID, { path: '/', secure: true });
        const body = JSON.stringify({ email: EMAIL, password: PASSWORD });
        const r = http.post(`${BASE}/login`, body, { headers: HEADERS });

        if (r.status === 200) return true;
        if (r.status === 202) {
            console.error('Device not trusted. Verify OTP once with remember_device=true for this DEVICE_ID.');
        } else {
            console.error('login failed:', r.status, safeBody(r.body));
        }
        return false;
    }

    return false;
}

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
                    { target: END_RATE, duration: `${STAGES_SEC}s` },
                    { target: END_RATE, duration: DURATION },
                ],
            },
        };
    }
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

// ===== Helpers =====
function toNum(v, def) {
    const n = Number(v);
    return Number.isFinite(n) && n > 0 ? n : def;
}
function toBool(v, def) {
    if (v === undefined) return def;
    const s = String(v).toLowerCase();
    if (s === 'true' || s === '1') return true;
    if (s === 'false' || s === '0') return false;
    return def;
}
function parseKV(s) {
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
function safeBody(b) {
    if (!b) return '';
    const s = String(b);
    return s.length > 200 ? s.slice(0, 200) + '…' : s;
}

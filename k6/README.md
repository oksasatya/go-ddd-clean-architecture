# k6 Load Test Scripts

This folder contains two k6 scripts:

- perf.js: Generic high-throughput test for any public endpoint.
- auth_protected.js: Scripted login per VU that obtains cookies, then hammers a protected endpoint.

## perf.js (public endpoints)

Examples:

- 1,000,000 requests (5k RPS for 200s):

```bash
k6 run -e URL=https://backend-api.oksasatya.dev/api/ip -e RATE=5000 -e DURATION=200s -e VUS=2000 -e MAX_VUS=10000 k6/perf.js
```

- Spike/ramp to 10k RPS:

```bash
k6 run -e URL=https://backend-api.oksasatya.dev/api/ip -e SCENARIO=ramping -e START_RATE=1000 -e END_RATE=10000 -e STAGES=60 -e DURATION=120s -e VUS=4000 -e MAX_VUS=20000 k6/perf.js
```

Env vars:
- URL, METHOD, BODY, CONTENT_TYPE, HEADERS
- RATE, DURATION, VUS, MAX_VUS
- SCENARIO=constant|ramping, START_RATE, END_RATE, STAGES
- INSECURE=true|false, SLEEP_MS, BUST_CACHE=true|false

## auth_protected.js (protected endpoints)

Bootstrap precedence per VU:
1) If REFRESH_TOKEN is provided, sets cookie, calls `/refresh` to get tokens.
2) Else if DEVICE_ID + EMAIL + PASSWORD provided (and device is pre-trusted), calls `/login` and expects 200.
3) Else aborts with guidance.

Examples:

- Using a refresh token:
```bash
k6 run -e BASE=https://backend-api.oksasatya.dev/api -e PROTECTED_PATH=/profile \
  -e REFRESH_TOKEN=PASTE_REFRESH_TOKEN \
  -e RATE=1000 -e DURATION=300s -e VUS=800 -e MAX_VUS=4000 k6/auth_protected.js
```

- Using a pre-trusted device:
```bash
k6 run -e BASE=https://backend-api.oksasatya.dev/api -e PROTECTED_PATH=/profile \
  -e EMAIL=user@example.com -e PASSWORD=YourPassword1! -e DEVICE_ID=PASTE_DEVICE_ID \
  -e RATE=1000 -e DURATION=300s -e VUS=800 -e MAX_VUS=4000 k6/auth_protected.js
```

Notes:
- The API uses HttpOnly cookies. k6 automatically stores and sends cookies per VU.
- If `/login` returns 202 requires_otp, trust the device once (OTP confirm with `remember_device=true`) and re-run.
- Rate limits may apply on auth endpoints; for raw throughput use a public route like `/api/ip`.


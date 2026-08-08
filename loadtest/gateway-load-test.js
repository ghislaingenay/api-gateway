// Load test for the API gateway (localhost:8080 by default).
//
// Prerequisites:
//   - k6 installed (https://k6.io/docs/get-started/installation/)
//   - the local stack running: `docker compose up --build` (gateway +
//     Keycloak)
//   - a seeded tenant/user, e.g. via `go run ./cmd/seed` (see README.md)
//
// Auth is against Keycloak directly (FEAT-012), not the gateway — the
// gateway has no /auth/login. setup() gets a token via the Resource Owner
// Password Credentials grant against the dedicated `client-loadtest`
// client (public, direct-access-grants-only, no PKCE/browser needed;
// see keycloak/realm-export.json), then reads the caller's tenant id from
// GET /auth/me since Keycloak doesn't know about gateway tenants.
//
// Run:
//   k6 run loadtest/gateway-load-test.js
//
// Override defaults (seeded local dev user) as needed:
//   k6 run \
//     -e BASE_URL=http://localhost:8080 \
//     -e KEYCLOAK_URL=http://localhost:8090 \
//     -e KEYCLOAK_REALM=api-gateway \
//     -e KEYCLOAK_CLIENT_ID=client-loadtest \
//     -e LOGIN_EMAIL=viewer@seed.test \
//     -e LOGIN_PASSWORD=password123 \
//     loadtest/gateway-load-test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const KEYCLOAK_URL = __ENV.KEYCLOAK_URL || 'http://localhost:8090';
const KEYCLOAK_REALM = __ENV.KEYCLOAK_REALM || 'api-gateway';
const KEYCLOAK_CLIENT_ID = __ENV.KEYCLOAK_CLIENT_ID || 'client-loadtest';
const LOGIN_EMAIL = __ENV.LOGIN_EMAIL || 'viewer@seed.test';
const LOGIN_PASSWORD = __ENV.LOGIN_PASSWORD || 'password123';

// By default k6's http_req_failed metric marks any non-2xx/3xx response as
// a failure, regardless of the checks below. 404 and 429 are legitimate,
// expected outcomes here (unknown order ID / gateway rate limiting under
// load), so they're excluded from http_req_failed too — otherwise the
// http_req_failed threshold trips on correct gateway behavior.
http.setResponseCallback(http.expectedStatuses(200, 201, 404, 429));

export const options = {
  stages: [
    { duration: '30s', target: 100 }, // ramp up
    { duration: '1m', target: 500 }, // sustain
    { duration: '30s', target: 1000 }, // spike
    { duration: '30s', target: 0 }, // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    // 429s from the gateway's rate limiter are expected under this load
    // profile, so this only guards against real (5xx/connection) failures.
    http_req_failed: ['rate<0.05'],
  },
};

// Runs once before the VUs start: logs in against Keycloak a single time
// and shares the token and tenant id, instead of every VU authenticating
// itself.
export function setup() {
  const tokenRes = http.post(
    `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token`,
    {
      grant_type: 'password',
      client_id: KEYCLOAK_CLIENT_ID,
      username: LOGIN_EMAIL,
      password: LOGIN_PASSWORD,
    },
    { headers: { 'Content-Type': 'application/x-www-form-urlencoded' } }
  );

  check(tokenRes, { 'keycloak login succeeded': (r) => r.status === 200 });
  if (tokenRes.status !== 200) {
    throw new Error(`setup: keycloak login failed with status ${tokenRes.status}: ${tokenRes.body}`);
  }

  const token = tokenRes.json('access_token');

  // The gateway never issues tenant ids; ask it directly which tenants
  // this seeded user belongs to (FEAT-012's GET /auth/me without
  // X-Tenant-ID).
  const meRes = http.get(`${BASE_URL}/auth/me`, {
    headers: { Authorization: `Bearer ${token}` },
  });

  check(meRes, { 'auth/me succeeded': (r) => r.status === 200 });
  if (meRes.status !== 200) {
    throw new Error(`setup: auth/me failed with status ${meRes.status}: ${meRes.body}`);
  }

  const tenants = meRes.json('tenants');
  if (!tenants || tenants.length === 0) {
    throw new Error(`setup: ${LOGIN_EMAIL} has no tenant memberships to load-test against`);
  }

  return { token, tenantId: tenants[0].tenant_id };
}

export default function (data) {
  const authHeaders = {
    headers: {
      Authorization: `Bearer ${data.token}`,
      'X-Tenant-ID': data.tenantId,
    },
  };

  const roll = Math.random();

  if (roll < 0.34) {
    // Baseline: unauthenticated, uncached.
    const res = http.get(`${BASE_URL}/health`);
    check(res, { 'health status is 200': (r) => r.status === 200 });
  } else if (roll < 0.67) {
    // Authenticated proxied read. Random UUID per request avoids skewing
    // results with response-cache hits; expect 404 from the mock
    // downstream unless the ID happens to exist, or 429 under heavy load.
    const res = http.get(`${BASE_URL}/api/orders/${uuidv4()}`, authHeaders);
    check(res, {
      'orders GET status is 200/404/429': (r) =>
        [200, 404, 429].includes(r.status),
    });
  } else {
    // Authenticated proxied write (naturally uncacheable).
    const res = http.post(
      `${BASE_URL}/api/orders`,
      JSON.stringify({
        customer_email: `loadtest-${uuidv4()}@example.com`,
        quantity: Math.ceil(Math.random() * 10),
      }),
      {
        headers: {
          ...authHeaders.headers,
          'Content-Type': 'application/json',
        },
      }
    );
    check(res, {
      'orders POST status is 200/201/429': (r) =>
        [200, 201, 429].includes(r.status),
    });
  }

  sleep(0.1);
}

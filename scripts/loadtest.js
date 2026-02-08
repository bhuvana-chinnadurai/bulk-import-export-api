// k6 Comprehensive Stress Test - ALL API Endpoints
// Usage: k6 run scripts/loadtest.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const exportThroughput = new Trend('export_rows_per_sec');
const errorRate = new Rate('errors');

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

// Store job IDs from imports for later status checks
let importJobIds = [];

// =============================================================================
// TEST CONFIGURATION
// =============================================================================
export const options = {
    scenarios: {
        // Health: 10 req/s
        health: {
            executor: 'constant-arrival-rate',
            rate: 10,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 5,
            maxVUs: 20,
            exec: 'testHealth',
            tags: { endpoint: 'health' },
        },

        // Export Users (streaming): 10 req/s
        export_users: {
            executor: 'constant-arrival-rate',
            rate: 10,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 10,
            maxVUs: 30,
            exec: 'testExportUsers',
            tags: { endpoint: 'export_users' },
        },

        // Export Articles (streaming): 5 req/s
        export_articles: {
            executor: 'constant-arrival-rate',
            rate: 5,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 5,
            maxVUs: 20,
            exec: 'testExportArticles',
            tags: { endpoint: 'export_articles' },
        },

        // POST Import: 5 req/s
        import_create: {
            executor: 'constant-arrival-rate',
            rate: 5,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 5,
            maxVUs: 20,
            exec: 'testImportCreate',
            tags: { endpoint: 'import_create' },
        },

        // GET Import Status: 10 req/s (starts after imports begin)
        import_status: {
            executor: 'constant-arrival-rate',
            rate: 10,
            timeUnit: '1s',
            duration: '25s',
            startTime: '5s',
            preAllocatedVUs: 5,
            maxVUs: 20,
            exec: 'testImportStatus',
            tags: { endpoint: 'import_status' },
        },

        // POST Export (async): 3 req/s
        export_async: {
            executor: 'constant-arrival-rate',
            rate: 3,
            timeUnit: '1s',
            duration: '30s',
            preAllocatedVUs: 3,
            maxVUs: 10,
            exec: 'testExportAsync',
            tags: { endpoint: 'export_async' },
        },
    },

    thresholds: {
        errors: ['rate<0.05'],
        http_req_duration: ['p(95)<5000'],
        'http_req_duration{endpoint:health}': ['p(95)<100'],
        'http_req_duration{endpoint:export_users}': ['p(95)<3000'],
        'http_req_duration{endpoint:export_articles}': ['p(95)<2000'],
        'http_req_duration{endpoint:import_create}': ['p(95)<1000'],
        'http_req_duration{endpoint:import_status}': ['p(95)<500'],
        'http_req_duration{endpoint:export_async}': ['p(95)<500'],
    },
};

// =============================================================================
// HELPERS
// =============================================================================

function randomString(length) {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < length; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return result;
}

function generateUUID() {
    return randomString(8) + '-' + randomString(4) + '-' + randomString(4) + '-' + randomString(4) + '-' + randomString(12);
}

function generateUsersCSV(numRecords) {
    let csv = 'id,email,name,role,active,created_at\n';
    const roles = ['admin', 'editor', 'viewer'];
    for (let i = 0; i < numRecords; i++) {
        const uuid = generateUUID();
        const role = roles[Math.floor(Math.random() * roles.length)];
        csv += `${uuid},user${i}_${randomString(6)}@test.com,User ${i},${role},true,2024-01-01T00:00:00Z\n`;
    }
    return csv;
}

// =============================================================================
// TEST FUNCTIONS
// =============================================================================

// 1. Health Check
export function testHealth() {
    const res = http.get(`${BASE_URL}/health`, { tags: { endpoint: 'health' } });
    const passed = check(res, { 'health: 200': (r) => r.status === 200 });
    errorRate.add(passed ? 0 : 1);
}

// 2. Export Users (streaming NDJSON)
export function testExportUsers() {
    const startTime = Date.now();
    const res = http.get(`${BASE_URL}/v1/exports?resource=users&format=ndjson`, {
        tags: { endpoint: 'export_users' }
    });
    const duration = Date.now() - startTime;

    const passed = check(res, {
        'export_users: 200': (r) => r.status === 200,
        'export_users: has data': (r) => r.body && r.body.length > 0,
    });

    if (passed && res.body) {
        const lines = res.body.split('\n').filter(l => l.length > 0).length;
        if (lines > 0 && duration > 0) {
            exportThroughput.add((lines / duration) * 1000);
        }
    }
    errorRate.add(passed ? 0 : 1);
}

// 3. Export Articles (streaming NDJSON)
export function testExportArticles() {
    const res = http.get(`${BASE_URL}/v1/exports?resource=articles&format=ndjson`, {
        tags: { endpoint: 'export_articles' }
    });
    const passed = check(res, { 'export_articles: 200': (r) => r.status === 200 });
    errorRate.add(passed ? 0 : 1);
}

// 4. Create Import Job (POST /v1/imports)
export function testImportCreate() {
    const csv = generateUsersCSV(50);
    const idempotencyKey = `k6-${Date.now()}-${randomString(8)}`;

    const res = http.post(`${BASE_URL}/v1/imports`,
        { resource: 'users', file: http.file(csv, 'users.csv', 'text/csv') },
        { headers: { 'Idempotency-Key': idempotencyKey }, tags: { endpoint: 'import_create' } }
    );

    const passed = check(res, {
        'import_create: 2xx': (r) => r.status >= 200 && r.status < 300,
        'import_create: has job_id': (r) => {
            try {
                const body = JSON.parse(r.body);
                if (body.job_id) {
                    importJobIds.push(body.job_id);
                }
                return body.job_id !== undefined;
            } catch { return false; }
        },
    });
    errorRate.add(passed ? 0 : 1);
}

// 5. Get Import Status (GET /v1/imports/{job_id})
export function testImportStatus() {
    // Use a known job ID or pick from recent imports
    const jobId = importJobIds.length > 0
        ? importJobIds[Math.floor(Math.random() * importJobIds.length)]
        : 'd5b91607-26cd-4311-8152-11d505e4cd11';

    const res = http.get(`${BASE_URL}/v1/imports/${jobId}`, {
        tags: { endpoint: 'import_status' }
    });

    const passed = check(res, {
        'import_status: 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
    errorRate.add(passed ? 0 : 1);
}

// 6. Async Export (POST /v1/exports)
export function testExportAsync() {
    const res = http.post(`${BASE_URL}/v1/exports`,
        JSON.stringify({ resource: 'users', format: 'json' }),
        { headers: { 'Content-Type': 'application/json' }, tags: { endpoint: 'export_async' } }
    );

    const passed = check(res, {
        'export_async: 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    errorRate.add(passed ? 0 : 1);
}

// =============================================================================
// LIFECYCLE HOOKS
// =============================================================================

export function setup() {
    console.log('');
    console.log('╔═══════════════════════════════════════════════════════════════════╗');
    console.log('║          COMPREHENSIVE API STRESS TEST - ALL ENDPOINTS            ║');
    console.log('╠═══════════════════════════════════════════════════════════════════╣');
    console.log('║  Endpoint                           │ Rate    │ Duration │ Total  ║');
    console.log('╠─────────────────────────────────────┼─────────┼──────────┼────────╣');
    console.log('║  GET  /health                       │ 10/s    │   30s    │  300   ║');
    console.log('║  GET  /v1/exports?resource=users    │ 10/s    │   30s    │  300   ║');
    console.log('║  GET  /v1/exports?resource=articles │  5/s    │   30s    │  150   ║');
    console.log('║  POST /v1/imports                   │  5/s    │   30s    │  150   ║');
    console.log('║  GET  /v1/imports/{job_id}          │ 10/s    │   25s    │  250   ║');
    console.log('║  POST /v1/exports (async)           │  3/s    │   30s    │   90   ║');
    console.log('╠─────────────────────────────────────┼─────────┼──────────┼────────╣');
    console.log('║  TOTAL                              │ 43/s    │   30s    │ 1240   ║');
    console.log('╚═══════════════════════════════════════════════════════════════════╝');
    console.log('');
    console.log(`Target: ${BASE_URL}`);

    const res = http.get(`${BASE_URL}/health`);
    if (res.status !== 200) throw new Error(`API not healthy: ${res.status}`);
    console.log('API is healthy. Starting comprehensive stress test...');
}

export function teardown() {
    console.log('');
    console.log('Comprehensive stress test completed!');
}

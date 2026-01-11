// k6 load testing script for HSM Service
// Run with: k6 run load-test.js
//
// Install k6: https://k6.io/docs/getting-started/installation/
// sudo apt install k6  # Ubuntu/Debian
// brew install k6      # macOS

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const encryptDuration = new Trend('encrypt_duration');
const decryptDuration = new Trend('decrypt_duration');
const totalOperations = new Counter('total_operations');

// Test configuration
export let options = {
  insecureSkipTLSVerify: true, // CRITICAL: Skip TLS verification for self-signed certs
  // mTLS configuration (required for HSM service)
  tlsAuth: [{
    domains: ['localhost', '127.0.0.1'],
    cert: open(__ENV.CLIENT_CERT || '../../pki/client/hsm-trading-client-1.crt'),
    key: open(__ENV.CLIENT_KEY || '../../pki/client/hsm-trading-client-1.key'),
  }],
  stages: [
    { duration: '1m', target: 50 },   // Warm-up: ramp to 50 users
    { duration: '3m', target: 100 },  // Ramp-up to 100 users
    { duration: '5m', target: 100 },  // Steady state at 100 users
    { duration: '2m', target: 200 },  // Spike to 200 users
    { duration: '5m', target: 200 },  // Maintain spike
    { duration: '2m', target: 100 },  // Scale back
    { duration: '3m', target: 50 },   // Cool down
    { duration: '1m', target: 0 },    // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<500', 'p(99)<1000'], // 95% < 500ms, 99% < 1s
    'http_req_failed': ['rate<0.01'],                  // <1% errors
    'errors': ['rate<0.01'],                           // <1% custom errors
    'encrypt_duration': ['p(95)<100', 'p(99)<200'],   // Encrypt: 95% < 100ms
    'decrypt_duration': ['p(95)<100', 'p(99)<200'],   // Decrypt: 95% < 100ms
  },
};

// Base URL (use environment variable or default)
const BASE_URL = __ENV.HSM_URL || 'https://localhost:8443';

// Test data
const testPayloads = [
  'SGVsbG8gV29ybGQh',                                              // Small: "Hello World!"
  'VGhpcyBpcyBhIG1lZGl1bSBzaXplZCBwYXlsb2FkIGZvciB0ZXN0aW5n',  // Medium
  'TG9uZ2VyIHBheWxvYWQgd2l0aCBtb3JlIGRhdGEgdG8gdGVzdCBwZXJmb3JtYW5jZSB1bmRlciB2YXJpb3VzIGNvbmRpdGlvbnMgYW5kIGxvYWRz', // Long
];

// IMPORTANT: Only use 'exchange-key' context because client cert has OU=Trading
// which only has access to 'exchange-key' (not '2fa')
const context = 'exchange-key';

export default function() {
  // Select random test data
  const plaintext = testPayloads[Math.floor(Math.random() * testPayloads.length)];

  // ========================================
  // Test 1: Encrypt endpoint
  // ========================================
  const encryptPayload = JSON.stringify({
    context: context,
    plaintext: plaintext,
  });

  const encryptParams = {
    headers: { 'Content-Type': 'application/json' },
    timeout: '10s',
  };

  const startEncrypt = Date.now();
  const encryptRes = http.post(`${BASE_URL}/encrypt`, encryptPayload, encryptParams);
  const encryptTime = Date.now() - startEncrypt;

  // DEBUG: Log failed requests
  if (encryptRes.status !== 200) {
    console.error(`Encrypt failed: status=${encryptRes.status}, body=${encryptRes.body}`);
  }

  const encryptOk = check(encryptRes, {
    'encrypt: status 200': (r) => r.status === 200,
    'encrypt: has ciphertext': (r) => r.json('ciphertext') !== undefined,
    'encrypt: has key_id': (r) => r.json('key_id') !== undefined,
  });

  if (encryptOk) {
    encryptDuration.add(encryptTime);
    totalOperations.add(1);

    // ========================================
    // Test 2: Decrypt endpoint (if encrypt succeeded)
    // ========================================
    const encryptData = encryptRes.json();
    const decryptPayload = JSON.stringify({
      context: context,
      ciphertext: encryptData.ciphertext,
      key_id: encryptData.key_id,
    });

    const decryptParams = {
      headers: { 'Content-Type': 'application/json' },
      timeout: '10s',
    };

    const startDecrypt = Date.now();
    const decryptRes = http.post(`${BASE_URL}/decrypt`, decryptPayload, decryptParams);
    const decryptTime = Date.now() - startDecrypt;

    const decryptOk = check(decryptRes, {
      'decrypt: status 200': (r) => r.status === 200,
      'decrypt: has plaintext': (r) => r.json('plaintext') !== undefined,
      'decrypt: plaintext matches': (r) => r.json('plaintext') === plaintext,
    });

    if (decryptOk) {
      decryptDuration.add(decryptTime);
      totalOperations.add(1);
    } else {
      errorRate.add(1);
    }
  } else {
    errorRate.add(1);
  }

  // ========================================
  // Test 3: Health endpoint (periodic check)
  // ========================================
  if (Math.random() < 0.1) { // 10% of iterations check health
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
      'health: status 200': (r) => r.status === 200,
      'health: service ok': (r) => r.json('status') === 'ok',
    });
  }

  // Small delay between iterations (realistic user behavior)
  // Adjusted to stay within rate limit: 200 users * 0.5 req/s = 100 req/s (rate limit)
  sleep(1.5 + Math.random()); // 1.5-2.5 seconds
}

// Summary at the end of test
export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'load-test-results.json': JSON.stringify(data),
  };
}

function textSummary(data, opts) {
  const indent = opts.indent || '';
  const enableColors = opts.enableColors || false;
  
  let summary = '\n' + indent + '='.repeat(60) + '\n';
  summary += indent + 'Load Test Summary\n';
  summary += indent + '='.repeat(60) + '\n\n';
  
  // Metrics
  const metrics = data.metrics;
  
  summary += indent + 'HTTP Metrics:\n';
  summary += indent + `  Total Requests: ${metrics.http_reqs?.values?.count || 0}\n`;
  summary += indent + `  Request Rate: ${(metrics.http_reqs?.values?.rate || 0).toFixed(2)} req/s\n`;
  summary += indent + `  Failed Requests: ${(metrics.http_req_failed?.values?.rate * 100 || 0).toFixed(2)}%\n`;
  summary += indent + `  Avg Duration: ${(metrics.http_req_duration?.values?.avg || 0).toFixed(2)}ms\n`;
  summary += indent + `  P95 Duration: ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms\n`;
  summary += indent + `  P99 Duration: ${(metrics.http_req_duration?.values['p(99)'] || 0).toFixed(2)}ms\n\n`;
  
  summary += indent + 'Custom Metrics:\n';
  summary += indent + `  Encrypt P95: ${(metrics.encrypt_duration?.values['p(95)'] || 0).toFixed(2)}ms\n`;
  summary += indent + `  Decrypt P95: ${(metrics.decrypt_duration?.values['p(95)'] || 0).toFixed(2)}ms\n`;
  summary += indent + `  Total Operations: ${metrics.total_operations?.values?.count || 0}\n`;
  summary += indent + `  Error Rate: ${(metrics.errors?.values?.rate * 100 || 0).toFixed(2)}%\n\n`;
  
  summary += indent + '='.repeat(60) + '\n';
  
  return summary;
}

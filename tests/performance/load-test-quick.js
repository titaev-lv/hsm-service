// Quick smoke test for k6 - minimal load testing
// Run with: k6 run tests/performance/load-test-quick.js
//
// This is a shorter version for quick validation (2 minutes instead of 22)

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const encryptDuration = new Trend('encrypt_duration');
const decryptDuration = new Trend('decrypt_duration');
const totalOperations = new Counter('total_operations');

// Quick test configuration (2 minutes total)
export const options = {
  insecureSkipTLSVerify: true, // CRITICAL: Skip TLS verification for self-signed certs
  // mTLS configuration (required for HSM service)
  tlsAuth: [{
    domains: ['localhost', '127.0.0.1'],
    cert: open(__ENV.CLIENT_CERT || '../../pki/client/hsm-trading-client-1.crt'),
    key: open(__ENV.CLIENT_KEY || '../../pki/client/hsm-trading-client-1.key'),
  }],
  stages: [
    { duration: '30s', target: 10 },  // Warm-up: ramp to 10 users
    { duration: '1m', target: 20 },   // Steady state at 20 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    'http_req_duration': ['p(95)<500', 'p(99)<1000'],
    'http_req_failed': ['rate<0.05'],  // <5% errors (more lenient for quick test)
    'errors': ['rate<0.05'],
    'encrypt_duration': ['p(95)<200', 'p(99)<500'],
    'decrypt_duration': ['p(95)<200', 'p(99)<500'],
  },
};

// Base URL
const BASE_URL = __ENV.HSM_URL || 'https://localhost:8443';

// Test data
const testPayloads = [
  'SGVsbG8gV29ybGQh',                    // "Hello World!"
  'VGVzdCBkYXRhIGZvciBsb2FkIHRlc3Rpbmc=', // "Test data for load testing"
];

// IMPORTANT: Only use 'exchange-key' context because client cert has OU=Trading
// which only has access to 'exchange-key' (not '2fa')
const context = 'exchange-key';

export default function() {
  const plaintext = testPayloads[Math.floor(Math.random() * testPayloads.length)];

  // Encrypt
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
    'encrypt: has ciphertext': (r) => {
      try {
        return r.json('ciphertext') !== undefined;
      } catch (e) {
        console.error(`Failed to parse encrypt response: ${r.body}`);
        return false;
      }
    },
    'encrypt: has key_id': (r) => {
      try {
        return r.json('key_id') !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  if (encryptOk) {
    encryptDuration.add(encryptTime);
    totalOperations.add(1);

    // Decrypt (if encrypt succeeded)
    try {
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
        'decrypt: has plaintext': (r) => {
          try {
            return r.json('plaintext') !== undefined;
          } catch (e) {
            console.error(`Failed to parse decrypt response: ${r.body}`);
            return false;
          }
        },
        'decrypt: plaintext matches': (r) => {
          try {
            return r.json('plaintext') === plaintext;
          } catch (e) {
            return false;
          }
        },
      });

      if (decryptOk) {
        decryptDuration.add(decryptTime);
        totalOperations.add(1);
      } else {
        errorRate.add(1);
      }
    } catch (e) {
      console.error(`Error during decrypt: ${e}`);
      errorRate.add(1);
    }
  } else {
    errorRate.add(1);
  }

  // Health check (10% of iterations)
  if (Math.random() < 0.1) {
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
      'health: status 200': (r) => r.status === 200,
    });
  }

  sleep(0.5 + Math.random() * 0.5); // 0.5-1.0 seconds
}

export function handleSummary(data) {
  console.log('\n' + '='.repeat(60));
  console.log('Quick Load Test Summary');
  console.log('='.repeat(60));
  
  const metrics = data.metrics;
  
  console.log('\nHTTP Metrics:');
  console.log(`  Total Requests: ${metrics.http_reqs?.values?.count || 0}`);
  console.log(`  Request Rate: ${(metrics.http_reqs?.values?.rate || 0).toFixed(2)} req/s`);
  console.log(`  Failed Requests: ${(metrics.http_req_failed?.values?.rate * 100 || 0).toFixed(2)}%`);
  console.log(`  Avg Duration: ${(metrics.http_req_duration?.values?.avg || 0).toFixed(2)}ms`);
  console.log(`  P95 Duration: ${(metrics.http_req_duration?.values['p(95)'] || 0).toFixed(2)}ms`);
  console.log(`  P99 Duration: ${(metrics.http_req_duration?.values['p(99)'] || 0).toFixed(2)}ms`);
  
  console.log('\nCustom Metrics:');
  console.log(`  Encrypt P95: ${(metrics.encrypt_duration?.values['p(95)'] || 0).toFixed(2)}ms`);
  console.log(`  Decrypt P95: ${(metrics.decrypt_duration?.values['p(95)'] || 0).toFixed(2)}ms`);
  console.log(`  Total Operations: ${metrics.total_operations?.values?.count || 0}`);
  console.log(`  Error Rate: ${(metrics.errors?.values?.rate * 100 || 0).toFixed(2)}%`);
  
  console.log('\n' + '='.repeat(60) + '\n');
  
  return {
    'stdout': '',
    'load-test-quick-results.json': JSON.stringify(data),
  };
}

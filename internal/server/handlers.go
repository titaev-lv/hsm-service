package server

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/titaev-lv/hsm-service/internal/hsm"
)

// Request/Response types
type EncryptRequest struct {
	Context   string `json:"context"`
	Plaintext string `json:"plaintext"` // base64
}

type EncryptResponse struct {
	Ciphertext string `json:"ciphertext"` // base64
	KeyID      string `json:"key_id"`
}

type DecryptRequest struct {
	Context    string `json:"context"`
	Ciphertext string `json:"ciphertext"` // base64
	KeyID      string `json:"key_id"`
}

type DecryptResponse struct {
	Plaintext string `json:"plaintext"` // base64
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthResponse struct {
	Status       string            `json:"status"`
	HSMAvailable bool              `json:"hsm_available"`
	KEKStatus    map[string]string `json:"kek_status"`
}

var (
	logAPI    = slog.With("module", "api")
	logACL    = slog.With("module", "acl")
	logCrypto = slog.With("module", "crypto")
)

// Helper functions
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logAPI.Error("error encoding JSON response", "error", err)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

// EncryptHandler handles /encrypt requests
func EncryptHandler(keyManager hsm.CryptoProvider, aclChecker *ACLChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST
		if r.Method != http.MethodPost {
			SetAuditErrorCode(w, "method_not_allowed")
			respondError(w, http.StatusMethodNotAllowed, "only POST allowed")
			return
		}

		// Limit request body size (DoS protection)
		const maxRequestSize = 1 * 1024 * 1024 // 1MB
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

		// 1. Parse request
		var req EncryptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			reqID := RequestIDFromContext(r.Context())
			logAPI.Warn("invalid JSON in request",
				"request_id", reqID,
				"path", r.URL.Path,
				"method", r.Method,
			)
			SetAuditErrorCode(w, "invalid_json")
			respondError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		SetAuditContext(w, req.Context)

		// 2. Extract client certificate
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			SetAuditErrorCode(w, "no_client_cert")
			respondError(w, http.StatusUnauthorized, "no client certificate")
			return
		}
		clientCert := r.TLS.PeerCertificates[0]
		clientCN := clientCert.Subject.CommonName

		// Extract OU from certificate
		var clientOU string
		if len(clientCert.Subject.OrganizationalUnit) > 0 {
			clientOU = clientCert.Subject.OrganizationalUnit[0]
		}

		// 3. ACL check
		if err := aclChecker.CheckAccess(clientCert, req.Context); err != nil {
			reqID := RequestIDFromContext(r.Context())
			logACL.Warn("ACL check failed",
				"request_id", reqID,
				"client_cn", clientCN,
				"context", req.Context,
				"error", err,
			)
			SetAuditErrorCode(w, "acl_denied")
			// Metrics: track ACL failure (security monitoring)
			RecordACLFailure()
			RecordRequest("/encrypt", clientCN, "acl_denied")
			respondError(w, http.StatusForbidden, err.Error())
			return
		}

		// 4. Decode plaintext from base64
		plaintext, err := base64.StdEncoding.DecodeString(req.Plaintext)
		if err != nil {
			SetAuditErrorCode(w, "invalid_base64")
			respondError(w, http.StatusBadRequest, "invalid base64 plaintext")
			return
		}
		// Zero plaintext memory after use (security: prevent memory dumps)
		defer func() {
			for i := range plaintext {
				plaintext[i] = 0
			}
		}()

		// 5. Encrypt with context, OU, and clientCN
		// AAD will be built based on key's mode (shared=OU, private=CN)
		ciphertext, keyID, err := keyManager.Encrypt(plaintext, req.Context, clientOU, clientCN)
		if err != nil {
			reqID := RequestIDFromContext(r.Context())
			logCrypto.Error("encryption failed",
				"request_id", reqID,
				"client_cn", clientCN,
				"context", req.Context,
				"error", err,
			)
			SetAuditErrorCode(w, "encrypt_error")
			// Metrics: track HSM error
			RecordHSMError("encrypt")
			RecordEncryptOp(req.Context, "failure")
			RecordRequest("/encrypt", clientCN, "error")
			respondError(w, http.StatusInternalServerError, "encryption failed")
			return
		}
		SetAuditKeyID(w, keyID)

		// Metrics: track successful encryption
		RecordEncryptOp(req.Context, "success")
		RecordRequest("/encrypt", clientCN, "success")

		// 7. Respond
		resp := EncryptResponse{
			Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
			KeyID:      keyID,
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// DecryptHandler handles /decrypt requests
func DecryptHandler(keyManager hsm.CryptoProvider, aclChecker *ACLChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only accept POST
		if r.Method != http.MethodPost {
			SetAuditErrorCode(w, "method_not_allowed")
			respondError(w, http.StatusMethodNotAllowed, "only POST allowed")
			return
		}

		// Limit request body size (DoS protection)
		const maxRequestSize = 1 * 1024 * 1024 // 1MB
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

		// 1. Parse request
		var req DecryptRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			reqID := RequestIDFromContext(r.Context())
			logAPI.Warn("invalid JSON in request",
				"request_id", reqID,
				"path", r.URL.Path,
				"method", r.Method,
			)
			SetAuditErrorCode(w, "invalid_json")
			respondError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		SetAuditContext(w, req.Context)
		SetAuditKeyID(w, req.KeyID)

		// 2. Extract client certificate
		if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
			SetAuditErrorCode(w, "no_client_cert")
			respondError(w, http.StatusUnauthorized, "no client certificate")
			return
		}
		clientCert := r.TLS.PeerCertificates[0]
		clientCN := clientCert.Subject.CommonName

		// 3. ACL check
		if err := aclChecker.CheckAccess(clientCert, req.Context); err != nil {
			reqID := RequestIDFromContext(r.Context())
			logACL.Warn("ACL check failed",
				"request_id", reqID,
				"client_cn", clientCN,
				"context", req.Context,
				"error", err,
			)
			SetAuditErrorCode(w, "acl_denied")
			// Metrics: track ACL failure (security monitoring)
			RecordACLFailure()
			RecordRequest("/decrypt", clientCN, "acl_denied")
			respondError(w, http.StatusForbidden, err.Error())
			return
		}

		// 4. Decode ciphertext from base64
		ciphertext, err := base64.StdEncoding.DecodeString(req.Ciphertext)
		if err != nil {
			SetAuditErrorCode(w, "invalid_base64")
			respondError(w, http.StatusBadRequest, "invalid base64 ciphertext")
			return
		}
		// Zero ciphertext memory after use (security: prevent memory dumps)
		defer func() {
			for i := range ciphertext {
				ciphertext[i] = 0
			}
		}()

		// Extract OU from certificate
		var clientOU string
		if len(clientCert.Subject.OrganizationalUnit) > 0 {
			clientOU = clientCert.Subject.OrganizationalUnit[0]
		}

		// 5. Decrypt with context, OU, clientCN, and keyID
		// AAD will be rebuilt based on key's mode (shared=OU, private=CN)
		plaintext, err := keyManager.Decrypt(ciphertext, req.Context, clientOU, clientCN, req.KeyID)
		if err != nil {
			reqID := RequestIDFromContext(r.Context())
			logCrypto.Warn("decryption failed",
				"request_id", reqID,
				"client_cn", clientCN,
				"context", req.Context,
				"key_id", req.KeyID,
				"error", err,
			)
			SetAuditErrorCode(w, "decrypt_error")
			// Metrics: track HSM error
			RecordHSMError("decrypt")
			RecordDecryptOp(req.Context, "failure")
			RecordRequest("/decrypt", clientCN, "error")
			// Don't expose internal error details
			respondError(w, http.StatusBadRequest, "decryption failed")
			return
		}
		// Zero plaintext memory after use (security: prevent memory dumps)
		defer func() {
			for i := range plaintext {
				plaintext[i] = 0
			}
		}()

		// Metrics: track successful decryption
		RecordDecryptOp(req.Context, "success")
		RecordRequest("/decrypt", clientCN, "success")

		// 7. Respond
		resp := DecryptResponse{
			Plaintext: base64.StdEncoding.EncodeToString(plaintext),
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// HealthHandler handles /health requests
func HealthHandler(keyManager hsm.CryptoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := HealthResponse{
			Status:       "healthy",
			HSMAvailable: true,
			KEKStatus:    make(map[string]string),
		}

		// Check each KEK
		for _, label := range keyManager.GetKeyLabels() {
			if keyManager.HasKey(label) {
				status.KEKStatus[label] = "available"
			} else {
				status.KEKStatus[label] = "unavailable"
				status.HSMAvailable = false
				status.Status = "degraded"
			}
		}

		// Return 200 if healthy, 503 if degraded
		httpStatus := http.StatusOK
		if status.Status != "healthy" {
			httpStatus = http.StatusServiceUnavailable
		}

		respondJSON(w, httpStatus, status)
	}
}

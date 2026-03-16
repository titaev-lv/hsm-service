package server

import "testing"

func TestMetricsHelpers_DoNotPanic(t *testing.T) {
	RecordRequest("/encrypt", "client-a", "success")
	RecordACLFailure()
	RecordRevocationFailure()
	RecordEncryptOp("exchange-key", "success")
	RecordEncryptOp("exchange-key", "failure")
	RecordDecryptOp("exchange-key", "success")
	RecordDecryptOp("exchange-key", "failure")
	RecordRateLimitHit("client-a")
	RecordHSMError("encrypt")
	RecordHSMError("decrypt")
}

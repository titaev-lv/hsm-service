package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/miekg/pkcs11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

type fakePKCS11Ctx struct {
	initializeErr  error
	getSlotListErr error
	slots          []uint
	tokenInfo      map[uint]pkcs11.TokenInfo
	tokenInfoErr   map[uint]error
	openSessionErr error
	sessionHandle  pkcs11.SessionHandle
	loginErr       error
	generateKeyErr error
	generatedKey   pkcs11.ObjectHandle

	initialized  bool
	destroyed    bool
	finalized    bool
	sessionOpen  bool
	sessionClose bool
	loggedIn     bool
	loggedOut    bool
}

func (f *fakePKCS11Ctx) Initialize() error {
	f.initialized = true
	return f.initializeErr
}

func (f *fakePKCS11Ctx) Destroy() {
	f.destroyed = true
}

func (f *fakePKCS11Ctx) Finalize() error {
	f.finalized = true
	return nil
}

func (f *fakePKCS11Ctx) GetSlotList(_ bool) ([]uint, error) {
	if f.getSlotListErr != nil {
		return nil, f.getSlotListErr
	}
	return f.slots, nil
}

func (f *fakePKCS11Ctx) GetTokenInfo(slotID uint) (pkcs11.TokenInfo, error) {
	if err, ok := f.tokenInfoErr[slotID]; ok {
		return pkcs11.TokenInfo{}, err
	}
	if info, ok := f.tokenInfo[slotID]; ok {
		return info, nil
	}
	return pkcs11.TokenInfo{}, nil
}

func (f *fakePKCS11Ctx) OpenSession(_ uint, _ uint) (pkcs11.SessionHandle, error) {
	if f.openSessionErr != nil {
		return 0, f.openSessionErr
	}
	f.sessionOpen = true
	if f.sessionHandle == 0 {
		f.sessionHandle = 1
	}
	return f.sessionHandle, nil
}

func (f *fakePKCS11Ctx) CloseSession(_ pkcs11.SessionHandle) error {
	f.sessionClose = true
	return nil
}

func (f *fakePKCS11Ctx) Login(_ pkcs11.SessionHandle, _ uint, _ string) error {
	if f.loginErr != nil {
		return f.loginErr
	}
	f.loggedIn = true
	return nil
}

func (f *fakePKCS11Ctx) Logout(_ pkcs11.SessionHandle) error {
	f.loggedOut = true
	return nil
}

func (f *fakePKCS11Ctx) GenerateKey(_ pkcs11.SessionHandle, _ []*pkcs11.Mechanism, _ []*pkcs11.Attribute) (pkcs11.ObjectHandle, error) {
	if f.generateKeyErr != nil {
		return 0, f.generateKeyErr
	}
	if f.generatedKey == 0 {
		f.generatedKey = 99
	}
	return f.generatedKey, nil
}

func installPKCS11FactoryHook(t *testing.T, fn func(lib string) pkcs11Context) {
	t.Helper()
	orig := newPKCS11Ctx
	newPKCS11Ctx = fn
	t.Cleanup(func() {
		newPKCS11Ctx = orig
	})
}

func testCreateCfg() *config.Config {
	return &config.Config{
		HSM: config.HSMConfig{
			PKCS11Lib: "/tmp/fake-pkcs11.so",
			SlotID:    "slot-0",
		},
	}
}

func TestFindSlotByLabel_GetSlotListError(t *testing.T) {
	fake := &fakePKCS11Ctx{getSlotListErr: errors.New("slot list failed")}
	_, err := findSlotByLabel(fake, "slot-0")
	if err == nil || !strings.Contains(err.Error(), "get slot list") {
		t.Fatalf("findSlotByLabel() error = %v", err)
	}
}

func TestFindSlotByLabel_NoSlots(t *testing.T) {
	fake := &fakePKCS11Ctx{slots: []uint{}}
	_, err := findSlotByLabel(fake, "slot-0")
	if err == nil || !strings.Contains(err.Error(), "no slots found") {
		t.Fatalf("findSlotByLabel() error = %v", err)
	}
}

func TestFindSlotByLabel_EmptyTokenLabelReturnsFirst(t *testing.T) {
	fake := &fakePKCS11Ctx{slots: []uint{7, 9}}
	slot, err := findSlotByLabel(fake, "")
	if err != nil {
		t.Fatalf("findSlotByLabel() unexpected error: %v", err)
	}
	if slot != 7 {
		t.Fatalf("slot = %d, want 7", slot)
	}
}

func TestFindSlotByLabel_MatchByTrimmedLabel(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots: []uint{3},
		tokenInfo: map[uint]pkcs11.TokenInfo{
			3: {Label: "slot-0   "},
		},
	}
	slot, err := findSlotByLabel(fake, "slot-0")
	if err != nil {
		t.Fatalf("findSlotByLabel() unexpected error: %v", err)
	}
	if slot != 3 {
		t.Fatalf("slot = %d, want 3", slot)
	}
}

func TestFindSlotByLabel_NotFoundIncludesAvailable(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots: []uint{1, 2},
		tokenInfo: map[uint]pkcs11.TokenInfo{
			1: {Label: "alpha"},
			2: {Label: "beta"},
		},
	}
	_, err := findSlotByLabel(fake, "missing")
	if err == nil || !strings.Contains(err.Error(), "available: alpha, beta") {
		t.Fatalf("findSlotByLabel() error = %v", err)
	}
}

func TestFindSlotByLabel_TokenInfoErrorSkipped(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots: []uint{1, 2},
		tokenInfoErr: map[uint]error{
			1: errors.New("token info failed"),
		},
		tokenInfo: map[uint]pkcs11.TokenInfo{
			2: {Label: "slot-0"},
		},
	}
	slot, err := findSlotByLabel(fake, "slot-0")
	if err != nil {
		t.Fatalf("findSlotByLabel() unexpected error: %v", err)
	}
	if slot != 2 {
		t.Fatalf("slot = %d, want 2", slot)
	}
}

func TestCreateKEKWithConfig_InitializeError(t *testing.T) {
	fake := &fakePKCS11Ctx{initializeErr: errors.New("init failed")}
	installPKCS11FactoryHook(t, func(_ string) pkcs11Context { return fake })

	err := createKEKWithConfig(testCreateCfg(), "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil || !strings.Contains(err.Error(), "pkcs11 initialize") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}

func TestCreateKEKWithConfig_OpenSessionError(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots:          []uint{1},
		tokenInfo:      map[uint]pkcs11.TokenInfo{1: {Label: "slot-0"}},
		openSessionErr: errors.New("open failed"),
	}
	installPKCS11FactoryHook(t, func(_ string) pkcs11Context { return fake })

	err := createKEKWithConfig(testCreateCfg(), "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil || !strings.Contains(err.Error(), "open session") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}

func TestCreateKEKWithConfig_LoginError(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots:       []uint{1},
		tokenInfo:   map[uint]pkcs11.TokenInfo{1: {Label: "slot-0"}},
		loginErr:    errors.New("login failed"),
		sessionOpen: true,
	}
	installPKCS11FactoryHook(t, func(_ string) pkcs11Context { return fake })

	err := createKEKWithConfig(testCreateCfg(), "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil || !strings.Contains(err.Error(), "login failed") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}

func TestCreateKEKWithConfig_GenerateKeyError(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots:          []uint{1},
		tokenInfo:      map[uint]pkcs11.TokenInfo{1: {Label: "slot-0"}},
		generateKeyErr: errors.New("gen failed"),
	}
	installPKCS11FactoryHook(t, func(_ string) pkcs11Context { return fake })

	err := createKEKWithConfig(testCreateCfg(), "1234", "kek-a", "exchange-key", 1, 256)
	if err == nil || !strings.Contains(err.Error(), "generate key") {
		t.Fatalf("createKEKWithConfig() error = %v", err)
	}
}

func TestCreateKEKWithConfig_Success(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots:        []uint{1},
		tokenInfo:    map[uint]pkcs11.TokenInfo{1: {Label: "slot-0"}},
		generatedKey: 777,
	}
	installPKCS11FactoryHook(t, func(_ string) pkcs11Context { return fake })

	out := captureStdout(t, func() {
		err := createKEKWithConfig(testCreateCfg(), "1234", "kek-a", "exchange-key", 2, 256)
		if err != nil {
			t.Fatalf("createKEKWithConfig() error: %v", err)
		}
	})

	if !strings.Contains(out, "Created KEK: kek-a") {
		t.Fatalf("expected success output, got: %s", out)
	}
	if !fake.initialized || !fake.destroyed || !fake.finalized {
		t.Fatalf("expected lifecycle calls, got init=%v destroy=%v finalize=%v", fake.initialized, fake.destroyed, fake.finalized)
	}
	if !fake.sessionClose || !fake.loggedOut {
		t.Fatalf("expected session cleanup, got close=%v logout=%v", fake.sessionClose, fake.loggedOut)
	}
}

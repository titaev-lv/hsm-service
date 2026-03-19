package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/miekg/pkcs11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

const defaultKeySize = 256

type pkcs11Context interface {
	Initialize() error
	Destroy()
	Finalize() error
	GetSlotList(tokenPresent bool) ([]uint, error)
	GetTokenInfo(slotID uint) (pkcs11.TokenInfo, error)
	OpenSession(slotID uint, flags uint) (pkcs11.SessionHandle, error)
	CloseSession(sh pkcs11.SessionHandle) error
	Login(sh pkcs11.SessionHandle, userType uint, pin string) error
	Logout(sh pkcs11.SessionHandle) error
	GenerateKey(sh pkcs11.SessionHandle, m []*pkcs11.Mechanism, temp []*pkcs11.Attribute) (pkcs11.ObjectHandle, error)
}

var newPKCS11Ctx = func(lib string) pkcs11Context {
	return pkcs11.New(lib)
}

func createKEKCommand(args []string) error {
	fs := flag.NewFlagSet("create-kek", flag.ContinueOnError)
	label := fs.String("label", "", "KEK label (required)")
	contextName := fs.String("context", "", "Context name (required)")
	version := fs.Int("version", 1, "Key version (default: 1)")
	keySize := fs.Int("size", defaultKeySize, "Key size in bits (128, 192, 256)")
	configPath := fs.String("config", getConfigPath(), "Path to config.yaml")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if *label == "" || *contextName == "" {
		return fmt.Errorf("--label and --context are required")
	}

	if *keySize != 128 && *keySize != 192 && *keySize != 256 {
		return fmt.Errorf("--size must be 128, 192, or 256")
	}

	pin := os.Getenv("HSM_PIN")
	if pin == "" {
		return fmt.Errorf("HSM_PIN environment variable not set")
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	return createKEKWithConfig(cfg, pin, *label, *contextName, *version, *keySize)
}

func createKEKWithConfig(cfg *config.Config, pin, label, contextName string, version, keySize int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pkcs11 initialize panic: %v", r)
		}
	}()

	p := newPKCS11Ctx(cfg.HSM.PKCS11Lib)
	if p == nil {
		return fmt.Errorf("pkcs11 initialize: failed to create PKCS#11 context")
	}
	if err := p.Initialize(); err != nil {
		return fmt.Errorf("pkcs11 initialize: %w", err)
	}
	defer p.Destroy()
	defer p.Finalize()

	slot, err := findSlotByLabel(p, cfg.HSM.SlotID)
	if err != nil {
		return err
	}

	session, err := p.OpenSession(slot, pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return fmt.Errorf("open session: %w", err)
	}
	defer p.CloseSession(session)

	if err := p.Login(session, pkcs11.CKU_USER, pin); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	defer p.Logout(session)

	mechanism := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_KEY_GEN, nil)}
	id, idHex := generateKeyID()

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_AES),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, keySize/8),
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, label),
		pkcs11.NewAttribute(pkcs11.CKA_ID, id),
		pkcs11.NewAttribute(pkcs11.CKA_TOKEN, true),
		pkcs11.NewAttribute(pkcs11.CKA_PRIVATE, true),
		pkcs11.NewAttribute(pkcs11.CKA_SENSITIVE, true),
		pkcs11.NewAttribute(pkcs11.CKA_ENCRYPT, true),
		pkcs11.NewAttribute(pkcs11.CKA_DECRYPT, true),
		pkcs11.NewAttribute(pkcs11.CKA_WRAP, true),
		pkcs11.NewAttribute(pkcs11.CKA_UNWRAP, true),
		pkcs11.NewAttribute(pkcs11.CKA_EXTRACTABLE, false),
	}

	keyHandle, err := p.GenerateKey(session, mechanism, template)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	log.Printf("Created KEK for context: %s", contextName)
	fmt.Printf("✓ Created KEK: %s (handle: %d, ID: %s, version: %d, created: %s)\n",
		label, keyHandle, idHex, version, createdAt)
	fmt.Printf("  Generated unique ID: %s\n", idHex)

	return nil
}

func generateKeyID() ([]byte, string) {
	timestamp := time.Now().UnixNano()
	id := make([]byte, 8)
	for i := 0; i < 8; i++ {
		id[7-i] = byte(timestamp >> (i * 8))
	}
	return id, hex.EncodeToString(id)
}

func findSlotByLabel(p pkcs11Context, tokenLabel string) (uint, error) {
	slots, err := p.GetSlotList(true)
	if err != nil {
		return 0, fmt.Errorf("get slot list: %w", err)
	}
	if len(slots) == 0 {
		return 0, fmt.Errorf("no slots found")
	}
	if tokenLabel == "" {
		return slots[0], nil
	}

	labels := make([]string, 0, len(slots))
	for _, slot := range slots {
		info, err := p.GetTokenInfo(slot)
		if err != nil {
			continue
		}
		label := strings.TrimSpace(info.Label)
		labels = append(labels, label)
		if label == tokenLabel {
			return slot, nil
		}
	}

	return 0, fmt.Errorf("token label %q not found (available: %s)", tokenLabel, strings.Join(labels, ", "))
}

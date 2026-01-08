package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/miekg/pkcs11"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: create-kek <label> <id-hex> <pin>")
		fmt.Println("Example: create-kek kek-exchange-v1 01 1234")
		os.Exit(1)
	}

	label := os.Args[1]
	idHex := os.Args[2]
	pin := os.Args[3]

	// Decode ID from hex
	id, err := hex.DecodeString(idHex)
	if err != nil {
		log.Fatalf("Invalid ID hex: %v", err)
	}

	// Load PKCS#11 library
	p := pkcs11.New("/usr/lib/softhsm/libsofthsm2.so")
	err = p.Initialize()
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	defer p.Destroy()
	defer p.Finalize()

	// Get slot list
	slots, err := p.GetSlotList(true)
	if err != nil {
		log.Fatalf("GetSlotList failed: %v", err)
	}
	if len(slots) == 0 {
		log.Fatal("No slots found")
	}

	// Open session
	session, err := p.OpenSession(slots[0], pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		log.Fatalf("OpenSession failed: %v", err)
	}
	defer p.CloseSession(session)

	// Login
	err = p.Login(session, pkcs11.CKU_USER, pin)
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	defer p.Logout(session)

	// Generate AES-256 key
	mechanism := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_AES_KEY_GEN, nil)}

	// Key template
	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_SECRET_KEY),
		pkcs11.NewAttribute(pkcs11.CKA_KEY_TYPE, pkcs11.CKK_AES),
		pkcs11.NewAttribute(pkcs11.CKA_VALUE_LEN, 32), // 256 bits
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
		log.Fatalf("GenerateKey failed: %v", err)
	}

	fmt.Printf("✓ Created KEK: %s (handle: %d, ID: %s)\n", label, keyHandle, idHex)
}

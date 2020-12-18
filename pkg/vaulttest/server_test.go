package vaulttest

import (
	"fmt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

var testDevServer *VaultServer
var testDevAddress string
var testServer *VaultServer
var testAddress string

func TestMain(m *testing.M) {
	setUp()

	code := m.Run()

	tearDown()

	os.Exit(code)
}

func setUp() {
	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatalf("Failed to get a free port on which to run the test vault server: %s", err)
	}

	testDevAddress = fmt.Sprintf("127.0.0.1:%d", port)

	testDevServer = NewVaultServer(testDevAddress)

	if !testDevServer.Running {
		testDevServer.DevServerStart()
	}

	port, err = freeport.GetFreePort()
	if err != nil {
		log.Fatalf("Failed to get a free port on which to run the test vault server: %s", err)
	}

	testAddress = fmt.Sprintf("127.0.0.1:%d", port)

	testServer = NewVaultServer(testAddress)

	if !testServer.Running {
		testServer.ServerStart()
	}
}

func tearDown() {
	testDevServer.ServerShutDown()
	testServer.ServerShutDown()
}

func TestVaultTestClient(t *testing.T) {
	client := testDevServer.VaultTestClient()

	secret, err := client.Logical().Read("secret/config")
	if err != nil {
		log.Printf("Failed to default secret config: %s", err)
		t.Fail()
	}

	assert.True(t, secret != nil, "We got a secret from the test vault server")
}

func TestVaultServer(t *testing.T) {
	client := testServer.VaultTestClient()

	secret, err := client.Logical().Read("sys/seal-status")
	if err != nil {
		log.Printf("Failed to check seal status: %s", err)
		t.Fail()
	}

	assert.True(t, secret != nil, "Failed to check seal status")
}

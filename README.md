# vaulttest

[![Current Release](https://img.shields.io/github/release/nikogura/vaulttest.svg)](https://img.shields.io/github/release/nikogura/vaulttest.svg)

[![Circle CI](https://circleci.com/gh/nikogura/vaulttest.svg?style=shield)](https://circleci.com/gh/nikogura/vaulttest)

[![Go Report Card](https://goreportcard.com/badge/github.com/nikogura/vaulttest)](https://goreportcard.com/report/github.com/nikogura/vaulttest)

[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/nikogura/vaulttest/pkg/vaulttest)

[![Coverage Status](https://codecov.io/gh/nikogura/vaulttest/branch/master/graph/badge.svg)](https://codecov.io/gh/nikogura/vaulttest)

Library for spinning up test instances of Hashicorp Vault for use in integration tests locally and in CI systems.

Hashicorp Vault is an awesome tool, but if your job is  *managing* it, you need more than pointing and clicking in a UI, or running vault commands against the server.

A much better way is to write some code that instruments your Vault in a predictable manner, but how does one *test* said code?  

What's really needed is a test Vault or better yet a fleet of them to test changes in parallel.

Unfortunately Hashicorp Vault's source code is not organized/ exported in a way to make it's internal api easily adapted to a fully code defined, in memory Vault dev server.

What we can do, however, is have this package spin one up- provided the `vault` binary is on the system somewhere.

The `vaulttest` package will find a free port, spin up vault in dev mode on that port, allow you to do your tests against it, and shut it down politely once you're done.

You can also spin up a 'real' vault - not just dev mode.  It's still using an in-memory storage system, but you can use this server to test out your vault api code and ensure that everything works.

# Prerequisites

* Hashicorp Vault, installed on your system somewhere in the PATH.  https://www.vaultproject.io/downloads.html

* This library: `go get github.com/nikogura/vaulttest`

# Dev Mode Usage

To use a dev mode vault, include the following in your test code:

    var testServer *vaulttest.VaultServer
    var testClient *api.Client

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

        testAddress := fmt.Sprintf("127.0.0.1:%d", port)

        testServer = vaulttest.NewVaultServer(testAddress)

        if !testServer.Running {
            testServer.DevServerStart()
            client := testServer.VaultTestClient()

            // set up some secret engines
            for _, endpoint := range []string{
                "prod",
                "stage",
                "dev",
            } {
                data := map[string]interface{}{
                    "type":        "kv-v2",
                    "description": "Production Secrets",
                }
                _, err := client.Logical().Write(fmt.Sprintf("sys/mounts/%s", endpoint), data)
                if err != nil {
                    log.Fatalf("Unable to create secret engine %q: %s", endpoint, err)
                }
            }

            // setup a PKI backend
            data := map[string]interface{}{
                "type":        "pki",
                "description": "PKI backend",
            }
            
            _, err := client.Logical().Write("sys/mounts/pki", data)
            if err != nil {
                log.Fatalf("Failed to create pki secrets engine: %s", err)
            }

            data = map[string]interface{}{
                "common_name": "test-ca",
                "ttl":         "43800h",
            }
            
            _, err = client.Logical().Write("pki/root/generate/internal", data)
            if err != nil {
                log.Fatalf("Failed to create root cert: %s", err)
            }

            data = map[string]interface{}{
                "max_ttl":         "24h",
                "ttl":             "24h",
                "allow_ip_sans":   true,
                "allow_localhost": true,
                "allow_any_name":  true,
            }
            
            _, err = client.Logical().Write("pki/roles/foo", data)
            if err != nil {
                log.Fatalf("Failed to create cert issuing role: %s", err)
            }

            data = map[string]interface{}{
                "type":        "cert",
                "description": "TLS Cert Auth endpoint",
            }

            _, err = client.Logical().Write("sys/auth/cert", data)
            if err != nil {
                log.Fatalf("Failed to enable TLS cert auth: %s", err)
            }
            
            ... Do other setup stuff ...
            
            testClient = client
        }
    }

    func tearDown() {
        if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
            os.Remove(tmpDir)
        }

        testServer.ServerShutDown()
    }
    
    func TestSecret(t *testing.T) {
        path := "dev/foo/bar"
        secret, err := testClient.Logical().Read(path)
        if err != nil {
            log.Printf("Unable to read %q: %s\n", path, err)
            t.Fail()
        }
        
        if secret == nil {
            log.Print("Nil Secret")
            t.fail() 
        }
        
        assert.True(t, secret.Data["foo"].(string) == "bar", "Successfully returned secret")
    }

# 'Normal' Mode Usage

If the code you're testing is intended to handle initialization and setup of a Vault server, you can spin up a 'normal' server too.  Of course if you do this, you're on your own for initialization and handling the unseal keys and initial root token.

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

        testAddress = fmt.Sprintf("127.0.0.1:%d", port)

        testServer = NewVaultServer(testDevAddress)

        if !testServer.Running {
            testServer.ServerStart()
        }
    }

    func tearDown() {
        testServer.ServerShutDown()
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


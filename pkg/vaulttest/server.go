package vaulttest

import (
	"bufio"
	"fmt"
	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-homedir"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// VaultServer Representation of a Vault Server
type VaultServer struct {
	Command       *exec.Cmd
	Running       bool
	UnsealKey     string
	RootToken     string
	UserToken     string
	UserTokenFile string
	Address       string
}

// NewVaultServer creates a VaultServer struct with either the address provided, or the default address of 127.0.0.1:8200
func NewVaultServer(address string) *VaultServer {
	if address == "" {
		address = "127.0.0.1:8200"
	}

	testServer := VaultServer{
		Address: address,
	}

	return &testServer
}

// VAULT_CONFIG_TEMPLATE A very minimal Vault config.  Not even a true 'template', since all we're doing is interpolating the address into the mix.
const VAULT_CONFIG_TEMPLATE = `ui = true
disable_mlock = true

listener "tcp" {
	address           = "%s"
	
	tls_disable       = "true"
}

storage "inmem" {}
`

// DevServerStart Starts a 'dev mode' server, and parses it's output to record the Unseal Keys and Root Token in the VaultServer object
func (t *VaultServer) DevServerStart() {
	// find the user's vault token file if it exists
	homeDir, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Unable to determine user's home dir: %s", err)
	}

	t.UserTokenFile = fmt.Sprintf("%s/.vault-token", homeDir)

	// read it into memory cos the test server is gonna overwrite it
	if _, err := os.Stat(t.UserTokenFile); !os.IsNotExist(err) {
		tokenBytes, err := ioutil.ReadFile(t.UserTokenFile)
		if err == nil {
			t.UserToken = string(tokenBytes)
		}
	}

	vault, err := exec.LookPath("vault")
	if err != nil {
		log.Fatal("'vault' is not installed and available on the path")
	}

	log.Printf("Starting Dev Server on %s\n", t.Address)
	t.Command = exec.Command(vault, "server", "-dev", "-dev-no-store-token", "-dev-listen-address", t.Address)

	t.Command.Stderr = os.Stderr
	out, err := t.Command.StdoutPipe()
	if err != nil {
		log.Fatalf("unable to connect to testserver's stdout: %s", err)
	}

	t.Command.Start()

	scanner := bufio.NewScanner(out)

	unsealPattern := regexp.MustCompile(`^Unseal Key:.+`)
	rootTokenPattern := regexp.MustCompile(`^Root Token:.+`)

	for t.UnsealKey == "" || t.RootToken == "" {
		scanner.Scan()
		line := scanner.Text()

		if t.UnsealKey == "" && unsealPattern.MatchString(line) {
			parts := strings.Split(line, ": ")
			if len(parts) > 1 {
				t.UnsealKey = parts[1]
				strings.TrimRight(t.UnsealKey, "\n")
				strings.TrimLeft(t.UnsealKey, " ")
			}

			continue
		}

		if t.RootToken == "" && rootTokenPattern.MatchString(line) {
			parts := strings.Split(line, ": ")
			if len(parts) > 1 {
				t.RootToken = parts[1]
				strings.TrimRight(t.RootToken, "\n")
				strings.TrimLeft(t.RootToken, " ")
			}

			continue
		}
	}

	t.Running = true
}

// ServerStart starts the VaultServer in normal mode.  This server is sealed by default.
func (t *VaultServer) ServerStart() {
	// find the user's vault token file if it exists
	homeDir, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Unable to determine user's home dir: %s", err)
	}

	t.UserTokenFile = fmt.Sprintf("%s/.vault-token", homeDir)

	// read it into memory cos the test server is gonna overwrite it
	if _, err := os.Stat(t.UserTokenFile); !os.IsNotExist(err) {
		tokenBytes, err := ioutil.ReadFile(t.UserTokenFile)
		if err == nil {
			t.UserToken = string(tokenBytes)
		}
	}

	vault, err := exec.LookPath("vault")
	if err != nil {
		log.Fatal("'vault' is not installed and available on the path")
	}

	log.Printf("Starting Server on %s\n", t.Address)
	// Vault expects a path to a config file.  This is a quick hack via a subshell to present the text as if it was, in fact, in a file, but in reality it's just a string.  Neat huh?
	config := fmt.Sprintf(VAULT_CONFIG_TEMPLATE, t.Address)
	configArg := fmt.Sprintf("-config=<(echo '%s')", config)

	command := fmt.Sprintf("%s server %s", vault, configArg)

	t.Command = exec.Command("bash", "-c", command)

	t.Command.Stderr = os.Stderr
	t.Command.Stdout = os.Stdout

	t.Command.Start()

	t.Running = true
}

// ServerShutDown shuts the server down
func (t *VaultServer) ServerShutDown() {
	if t.Running {
		t.Command.Process.Kill()
	}

	// restore the user's vault token when we're done.
	if t.UserToken != "" {
		_ = ioutil.WriteFile(t.UserTokenFile, []byte(t.UserToken), 0600)
	}
}

// VaultTestClient returns a configured vault client for the test vault server.  By default the client returned has the root token for the test vault instance set.  If you want something else, you will need to reconfigure it.
func (t *VaultServer) VaultTestClient() *api.Client {
	config := api.DefaultConfig()

	err := config.ReadEnvironment()
	if err != nil {
		log.Fatalf("failed to inject environment into test vault client config")
	}

	config.Address = fmt.Sprintf("http://%s", t.Address)

	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("failed to create test vault api client: %s", err)
	}

	client.SetToken(t.RootToken)

	return client
}

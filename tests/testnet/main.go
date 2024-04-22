package main

// Notes(Tom):
//	- Using cometbft cli since it is easier than importing the libraries for now.
//	-

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

const NODE_COUNT = 10
const DIR_BASE = "/tmp/test"
const INCREMENT_IPS = false

// Won't necessarily be compatible with OS.
// Made to work internally only.
func ipToString(ip uint32) string {
	ret := ""
	ret += strconv.Itoa(int((ip >> 24) & 0xff))
	ret += "."
	ret += strconv.Itoa(int((ip >> 16) & 0xff))
	ret += "."
	ret += strconv.Itoa(int((ip >> 8) & 0xff))
	ret += "."
	ret += strconv.Itoa(int((ip >> 0) & 0xff))
	return ret
}

// Won't necessarily be compatible with OS.
// Made to work internally only.
func ipToInt(ip string) uint32 {
	chunks := strings.Split(ip, ".")

	var ret uint32
	a, _ := strconv.Atoi(chunks[3])
	b, _ := strconv.Atoi(chunks[2])
	c, _ := strconv.Atoi(chunks[1])
	d, _ := strconv.Atoi(chunks[0])

	ret = 0
	ret |= uint32((a << 0))
	ret |= uint32((b << 8))
	ret |= uint32((c << 16))
	ret |= uint32((d << 24))

	return ret
}

func cbftInit(absoluteDirectory string) {
	cmd := exec.Command("cometbft", "init", "--home", absoluteDirectory)
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func cbftGetId(absoluteDirectory string) string {
	cmd := exec.Command("cometbft", "show-node-id", "--home", absoluteDirectory)
	buf := new(bytes.Buffer)
	cmd.Stderr = buf

	out, err := cmd.Output()
	if err != nil {
		fmt.Println(buf.String())
		panic(err)
	}

	return strings.ReplaceAll(string(out), "\n", "")
}

func strInt(s string) int {
	ret, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}

	return ret
}

func intStr(i int) string {
	ret := strconv.Itoa(i)

	return ret
}

func main() {
	// Check if cometbft command is installed?

	var ip uint32
	if len(os.Args) > 1 {
		ip = ipToInt(os.Args[1])
	} else {
		resp, err := http.Get("https://ifconfig.me")
		if err != nil {
			panic(err)
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		ip = ipToInt(buf.String())
	}

	fmt.Println("Running on ip", ipToString(ip))
	bootstrapNodeIp := ip
	seedNodeIp := bootstrapNodeIp

	// Load template config.
	configTemplateBytes, err := os.ReadFile("config-template.toml")
	configTemplate := string(configTemplateBytes)

	if err != nil {
		panic(err)
	}

	// Set up bootstrap node.
	fmt.Println("Setting up configs")

	// Set up seed node.
	seedAddress := ""
	seedDir := DIR_BASE + "/node-seed"
	portCurrent := 26654
	{
		cbftInit(seedDir + "/cbft")
		configSeed := strings.Clone(configTemplate)
		configSeed = strings.ReplaceAll(configSeed, "{{ ip }}", "0.0.0.0")
		configSeed = strings.ReplaceAll(configSeed, "{{ ip-public }}", ipToString(seedNodeIp))
		configSeed = strings.ReplaceAll(configSeed, "{{ port-p2p }}", intStr(portCurrent))
		seedAddress = cbftGetId(seedDir+"/cbft") + "@" + ipToString(seedNodeIp) + ":" + intStr(portCurrent)
		portCurrent += 1
		configSeed = strings.ReplaceAll(configSeed, "{{ port-rpc }}", intStr(portCurrent))
		portCurrent += 1
		configSeed = strings.ReplaceAll(configSeed, "{{ port-abci }}", intStr(portCurrent))
		portCurrent += 1
		configSeed = strings.ReplaceAll(configSeed, "{{ persistent_peers }}", "")
		configSeed = strings.ReplaceAll(configSeed, "{{ seed_mode }}", "true")
		configSeed = strings.ReplaceAll(configSeed, "{{ listen_prometheus }}", "false")
		configSeed = strings.ReplaceAll(configSeed, "{{ seeds }}", "")

		os.WriteFile(seedDir+"/cbft/config/config.toml", []byte(configSeed), 0o777)

		setupYamlConfig(seedDir)
	}
	fmt.Println("Wrote seed node config")

	genesisFile := ""
	bootstrapDir := DIR_BASE + "/node-bootstrap"
	{
		// Generate keys.
		// Get a copy of the public id.
		// Actually just steal genesis.json and paste on other nodes.

		cbftInit(bootstrapDir + "/cbft")

		// Paste config and replace values.
		configBootstrap := strings.Clone(configTemplate)
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ ip }}", "0.0.0.0")
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ ip-public }}", ipToString(seedNodeIp))
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ port-p2p }}", intStr(portCurrent))
		portCurrent += 1
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ port-rpc }}", intStr(portCurrent))
		portCurrent += 1
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ port-abci }}", intStr(portCurrent))
		portCurrent += 1
		portCurrent += 1
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ persistent_peers }}", "")
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ seed_mode }}", "false")
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ listen_prometheus }}", "true")
		configBootstrap = strings.ReplaceAll(configBootstrap, "{{ seeds }}", seedAddress)

		os.WriteFile(bootstrapDir+"/cbft/config/config.toml", []byte(configBootstrap), 0o777)

		genesisFileBytes, err := os.ReadFile(bootstrapDir + "/cbft/config/genesis.json")
		if err != nil {
			panic(err)
		}

		setupYamlConfig(bootstrapDir)
		genesisFile = string(genesisFileBytes)
	}
	fmt.Println("Wrote bootstrap node config")

	// Copy the genesis file to the seed node.
	os.WriteFile(seedDir+"/cbft/config/genesis.json", []byte(genesisFile), 0o777)
	ipCurrent := seedNodeIp

	for i := 0; i < NODE_COUNT; i++ {
		nodeDir := DIR_BASE + "/node-" + strconv.Itoa(i)
		cbftInit(nodeDir + "/cbft")

		// Paste config and replace values.
		configNode := strings.Clone(configTemplate)
		configNode = strings.ReplaceAll(configNode, "{{ ip }}", "0.0.0.0")
		configNode = strings.ReplaceAll(configNode, "{{ ip-public }}", ipToString(seedNodeIp))

		// Set up a persistent peer?
		// HACK: Test only seed node.
		if true {
			configNode = strings.ReplaceAll(configNode, "{{ persistent_peers }}", "")
		} else {
			// Get the last person's info.
			panic("This should not be running")
			// id := cbftGetId(DIR_BASE + "/node-" + strconv.Itoa(i-1) + "/cbft")

			// peerAddress := id + "@" + ipToString(ipCurrent-1) + ":26656"
			// configNode = strings.ReplaceAll(configNode, "{{ persistent_peers }}", peerAddress)
		}
		configNode = strings.ReplaceAll(configNode, "{{ seed_mode }}", "false")
		configNode = strings.ReplaceAll(configNode, "{{ listen_prometheus }}", "false")
		configNode = strings.ReplaceAll(configNode, "{{ seeds }}", seedAddress)

		configNode = strings.ReplaceAll(configNode, "{{ port-p2p }}", intStr(portCurrent))
		portCurrent += 1
		configNode = strings.ReplaceAll(configNode, "{{ port-rpc }}", intStr(portCurrent))
		portCurrent += 1
		configNode = strings.ReplaceAll(configNode, "{{ port-abci }}", intStr(portCurrent))
		portCurrent += 1

		os.WriteFile(nodeDir+"/cbft/config/config.toml", []byte(configNode), 0o777)

		setupYamlConfig(nodeDir)
		os.WriteFile(nodeDir+"/cbft/config/genesis.json", []byte(genesisFile), 0o777)
		ipCurrent += 1

		fmt.Println("Made node: ", i)
	}

	fmt.Println("Compiling go program.")
	cmd := exec.Command("go", "build", "-o", "core", "../..")
	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	// Launch all the nodes as processes.
	fmt.Println("Running all nodes as processes.")

	cmds := make([]*exec.Cmd, NODE_COUNT+2)
	{
		// Run first node.
		cmds[0] = exec.Command("./core", "-config", bootstrapDir+"/config.yaml")
		cmds[0].Stdout = os.Stdout
		cmds[0].Stderr = os.Stderr

		// Run seed node.
		cmds[1] = exec.Command("./core", "-config", seedDir+"/config.yaml")
		// cmds[1].Stdout = os.Stdout
		// cmds[1].Stderr = os.Stderr

		// Run all other nodes.
		for i := 0; i < NODE_COUNT; i++ {
			nodeDir := DIR_BASE + "/node-" + strconv.Itoa(i)
			cmds[i+2] = exec.Command("./core", "-config", nodeDir+"/config.yaml")
		}
		// cmds[3].Stdout = os.Stdout
		// cmds[3].Stderr = os.Stderr
	}

	for i := range cmds {
		if cmds[i] != nil {
			fmt.Println("Started: ", i)
			err = cmds[i].Start()
			if err != nil {
				panic(err)
			}
		}
	}

	// Subscribe to sig int and cancel when done.
	exitSignal := make(chan os.Signal, 1)
	signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	<-exitSignal
	fmt.Println("Got exit signal, leaving!")

	for i := range cmds {
		// Docker can't pull this off for some reason.
		if cmds[i] != nil {
			if cmds[i].Process != nil {
				cmds[i].Process.Signal(os.Kill)
			}
		}
	}
	fmt.Println("Finished killing cmds")

	fmt.Println("Removing directories")
	err = os.RemoveAll(DIR_BASE)
	fmt.Println("Removed directories")
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}

func setupYamlConfig(absoluteDirectory string) {
	const yaml = `
p2p:
  # 0.0.0.0 = localhost
  addr: 0.0.0.0
  # 0 = random port
  port: 0
  groupName: xnode
  peerLimit: 50
bft:
  homeDir: {{ bft-home-dir }}
db:
  username: username
  password: password
  port: 5432
  dbName: nodedata
  url: 127.0.0.1
log:
  development: true
  encoding: json
  info:
    toStdout: true
    toFile: true
    fileName: logs/core-info.log
    maxSize: 10
    maxAge: 7
    maxBackups: 10
  error:
    toStderr: true
    toFile: true
    fileName: logs/core-error.log
    maxSize: 10
    maxAge: 7
    maxBackups: 10
nft:
  trackerData: /core/default-cometbft-home/data/nft-tracked
  rpcAddress: https://rpc.ankr.com/eth_sepolia
  unlimitedRPC: false
`

	fixedYaml := strings.Clone(yaml)
	fixedYaml = strings.ReplaceAll(fixedYaml, "{{ bft-home-dir }}", absoluteDirectory+"/cbft")

	os.WriteFile(absoluteDirectory+"/config.yaml", []byte(fixedYaml), 0o777)
}

// Copyright 2016 The go-PaloAltoAi Authors
// This file is part of go-PaloAltoAi.
//
// go-PaloAltoAi is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-PaloAltoAi is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-PaloAltoAi. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaloAltoAi/go-PaloAltoAi/params"
)

const (
	ipcAPIs  = "admin:1.0 debug:1.0 paa:1.0 paaash:1.0 miner:1.0 net:1.0 personal:1.0 rpc:1.0 shh:1.0 txpool:1.0 web3:1.0"
	httpAPIs = "paa:1.0 net:1.0 rpc:1.0 web3:1.0"
)

// Tests that a node embedded within a console can be started up properly and
// then terminated by closing the input stream.
func TestConsoleWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"

	// Start a gpaa console, make sure it's cleaned up and terminate the console
	gpaa := runGpaa(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--paaerbase", coinbase, "--shh",
		"console")

	// Gather all the infos the welcome message needs to contain
	gpaa.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	gpaa.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	gpaa.SetTemplateFunc("gover", runtime.Version)
	gpaa.SetTemplateFunc("gpaaver", func() string { return params.VersionWithMeta })
	gpaa.SetTemplateFunc("niltime", func() string { return time.Unix(0, 0).Format(time.RFC1123) })
	gpaa.SetTemplateFunc("apis", func() string { return ipcAPIs })

	// Verify the actual welcome message to the required template
	gpaa.Expect(`
Welcome to the Gpaa JavaScript console!

instance: Gpaa/v{{gpaaver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{.Paaerbase}}
at block: 0 ({{niltime}})
 datadir: {{.Datadir}}
 modules: {{apis}}

> {{.InputLine "exit"}}
`)
	gpaa.ExpectExit()
}

// Tests that a console can be attached to a running node via various means.
func TestIPCAttachWelcome(t *testing.T) {
	// Configure the instance for IPC attachement
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	var ipc string
	if runtime.GOOS == "windows" {
		ipc = `\\.\pipe\gpaa` + strconv.Itoa(trulyRandInt(100000, 999999))
	} else {
		ws := tmpdir(t)
		defer os.RemoveAll(ws)
		ipc = filepath.Join(ws, "gpaa.ipc")
	}
	// Note: we need --shh because testAttachWelcome checks for default
	// list of ipc modules and shh is included there.
	gpaa := runGpaa(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--paaerbase", coinbase, "--shh", "--ipcpath", ipc)

	time.Sleep(2 * time.Second) // Simple way to wait for the RPC endpoint to open
	testAttachWelcome(t, gpaa, "ipc:"+ipc, ipcAPIs)

	gpaa.Interrupt()
	gpaa.ExpectExit()
}

func TestHTTPAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P
	gpaa := runGpaa(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--paaerbase", coinbase, "--rpc", "--rpcport", port)

	time.Sleep(2 * time.Second) // Simple way to wait for the RPC endpoint to open
	testAttachWelcome(t, gpaa, "http://localhost:"+port, httpAPIs)

	gpaa.Interrupt()
	gpaa.ExpectExit()
}

func TestWSAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P

	gpaa := runGpaa(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--paaerbase", coinbase, "--ws", "--wsport", port)

	time.Sleep(2 * time.Second) // Simple way to wait for the RPC endpoint to open
	testAttachWelcome(t, gpaa, "ws://localhost:"+port, httpAPIs)

	gpaa.Interrupt()
	gpaa.ExpectExit()
}

func testAttachWelcome(t *testing.T, gpaa *testgpaa, endpoint, apis string) {
	// Attach to a running gpaa note and terminate immediately
	attach := runGpaa(t, "attach", endpoint)
	defer attach.ExpectExit()
	attach.CloseStdin()

	// Gather all the infos the welcome message needs to contain
	attach.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	attach.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	attach.SetTemplateFunc("gover", runtime.Version)
	attach.SetTemplateFunc("gpaaver", func() string { return params.VersionWithMeta })
	attach.SetTemplateFunc("paaerbase", func() string { return gpaa.Paaerbase })
	attach.SetTemplateFunc("niltime", func() string { return time.Unix(0, 0).Format(time.RFC1123) })
	attach.SetTemplateFunc("ipc", func() bool { return strings.HasPrefix(endpoint, "ipc") })
	attach.SetTemplateFunc("datadir", func() string { return gpaa.Datadir })
	attach.SetTemplateFunc("apis", func() string { return apis })

	// Verify the actual welcome message to the required template
	attach.Expect(`
Welcome to the Gpaa JavaScript console!

instance: Gpaa/v{{gpaaver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{paaerbase}}
at block: 0 ({{niltime}}){{if ipc}}
 datadir: {{datadir}}{{end}}
 modules: {{apis}}

> {{.InputLine "exit" }}
`)
	attach.ExpectExit()
}

// trulyRandInt generates a crypto random integer used by the console tests to
// not clash network ports with other tests running cocurrently.
func trulyRandInt(lo, hi int) int {
	num, _ := rand.Int(rand.Reader, big.NewInt(int64(hi-lo)))
	return int(num.Int64()) + lo
}

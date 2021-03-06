// Copyright © 2016 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/zerobotlabs/nestor-cli/Godeps/_workspace/src/github.com/fatih/color"
	"github.com/zerobotlabs/nestor-cli/Godeps/_workspace/src/github.com/mitchellh/go-homedir"
	"github.com/zerobotlabs/nestor-cli/Godeps/_workspace/src/github.com/peterh/liner"
	"github.com/zerobotlabs/nestor-cli/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/zerobotlabs/nestor-cli/app"
	"github.com/zerobotlabs/nestor-cli/exec"
	"github.com/zerobotlabs/nestor-cli/login"
)

const nestorRoot string = ".nestor"
const historyFileName string = "history"

var historyFile string

func init() {
	h, err := homedir.Dir()
	if err != nil {
		panic(err)
	}

	historyFile = path.Join(h, nestorRoot, historyFileName)
	_, err = os.Stat(historyFile)
	if err != nil && os.IsNotExist(err) {
		ioutil.WriteFile(historyFile, []byte(""), 0644)
	}
}

var quitPattern *regexp.Regexp = regexp.MustCompile("^(quit|exit)$")
var setEnvPattern *regexp.Regexp = regexp.MustCompile("^nestor\\.setenv ([^=]+)=(.*?)$")
var getEnvPattern *regexp.Regexp = regexp.MustCompile("^nestor\\.getenv\\s*(.*?)$")

func saveHistory(line *liner.State, hf *os.File) {
	line.WriteHistory(hf)
	line.Close()
	hf.Close()
}

func runShell(cmd *cobra.Command, args []string) {
	var l *login.LoginInfo
	var a app.App

	// Check if you are logged in first
	if l = login.SavedLoginInfo(); l == nil {
		color.Red("You are not logged in. To login, type \"nestor login\"\n")
		os.Exit(1)
	}

	// Check if you have a valid nestor.json file
	nestorJsonPath, err := pathToNestorJson(args)
	if err != nil {
		color.Red("Could not find nestor.json in the path specified\n")
		os.Exit(1)
	}

	a.ManifestPath = nestorJsonPath

	err = a.ParseManifest()
	if err != nil {
		color.Red("%s\n", err.Error())
		os.Exit(1)
	}

	// Check if existing app exists and if so, then we should be making calls to the "UPDATE" function
	// We are ignoring the error for now but at some point we will have to show an error that is not annoying
	err = a.Hydrate(l)
	if err != nil {
		color.Red("Error fetching details for power\n")
	}

	if a.Id == 0 {
		color.Red("You haven't saved your power yet. Run `nestor save` before you can test your power\n")
		os.Exit(1)
	}

	ok := false

	line := liner.NewLiner()

	line.SetCtrlCAborts(true)
	hf, err := os.OpenFile(historyFile, os.O_RDWR, 0644)
	if err != nil {
		color.Red("Unexpected error opening shell\n")
		line.Close()
		os.Exit(1)
	}

	line.ReadHistory(hf)

	for !ok {
		if command, err := line.Prompt("nestor> "); err == nil {
			command = strings.TrimSpace(command)

			if command != "" {
				line.AppendHistory(command)
			}
			switch {
			case quitPattern.MatchString(command):
				fmt.Println("Goodbye!")
				saveHistory(line, hf)
				os.Exit(1)
			case setEnvPattern.MatchString(command):
				matches := setEnvPattern.FindAllStringSubmatch(command, -1)
				resp, err := a.UpdateEnv(l, matches[0][1], matches[0][2])
				if err != nil {
					color.Red("There was an error setting environment variable %s for your power", matches[0][1])
				} else {
					fmt.Printf("Set %s to %s\n", matches[0][1], resp)
				}
			case getEnvPattern.MatchString(command):
				matches := getEnvPattern.FindAllStringSubmatch(command, -1)
				table, err := a.GetEnv(l, matches[0][1])
				if err != nil {
					color.Red("There was an error getting environment variable %s for your power", matches[0][1])
				} else {
					table.Render()
					fmt.Printf("\n")
				}
			case command == "":
				continue
			default:
				output := exec.Output{}
				err := output.Exec(&a, l, command)
				if err != nil {
					color.Red("unexpected error while running your power. Please try again later or contact hello@asknestor.me\n", err)
				}
				if output.Logs != "" {
					color.Yellow(output.Logs)
				}
				if len(output.ToSuggest) > 0 {
					suggestion := output.ToSuggest[0]
					fmt.Println("Oops, did you mean `" + suggestion + "`?")
				} else {
					for _, send := range output.ToSend {
						fmt.Println(send.ToString())
					}
				}
			}
		}
	}

	saveHistory(line, hf)
}

// shellCmd represents the shell command
var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open an interactive shell to debug your Nestor Power",
	Run:   runShell,
}

func init() {
	RootCmd.AddCommand(shellCmd)
}

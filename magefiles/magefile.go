package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// ParamLookupTable looks up the run parameters for a given challenge name + part name.
type ParamLookupTable = map[string]map[string][]string

var (
	paramLookup = ParamLookupTable{
		"echo": map[string][]string{
			"-": {"--node-count", "1", "--time-limit", "10"},
		},
		"unique-ids": map[string][]string{
			"-": {"--time-limit", "30", "--rate", "1000", "--node-count", "3", "--availability", "total", "--nemesis", "partition"},
		},
		"broadcast": map[string][]string{
			"3a": {"--node-count", "1", "--time-limit", "20", "--rate", "10"},
			"3b": {"--node-count", "5", "--time-limit", "20", "--rate", "10"},
			"3c": {"--node-count", "5", "--time-limit", "20", "--rate", "10", "--nemesis", "partition"},
		},
	}
)

// Ls lists the available challenge:part combos to run. "{challenge}:-" means there's only one part to the challenge.
func Ls() error {
	for challenge := range paramLookup {
		var parts []string
		for part := range paramLookup[challenge] {
			parts = append(parts, part)
		}
		sort.Strings(parts)
		_, _ = fmt.Fprintf(os.Stderr, "    - %s [%s]\n", challenge, strings.Join(parts, ","))
	}
	return nil
}

func lookupPartParams(name, part string) []string {
	params, ok := paramLookup[name][part]
	if !ok {
		panic(fmt.Sprintf("found no params for name: %s, part: %s -- run `mage ls` to see options", name, part))
	}
	return params
}

func lookupParams(name string) map[string][]string {
	paramMap, ok := paramLookup[name]
	if !ok {
		panic(fmt.Sprintf("found no params for name: %s -- run `mage ls` to see options", name))
	}
	return paramMap
}

func binaryLoc(name string) string {
	return fmt.Sprintf("./target/%s", name)
}

func writeLog(msg string) {
	_, _ = fmt.Fprintf(os.Stderr, fmt.Sprintf("MAGE: %s", msg))
}

// Build builds Go binary for {name}
func Build(name string) error {
	writeLog(fmt.Sprintf("building binary '%s'\n", name))
	srcDir := fmt.Sprintf("./%s", name)
	return sh.RunV("go", "build", "-o", binaryLoc(name), srcDir)
}

type Run mg.Namespace

// Challenge runs the given {challenge}
func (Run) Challenge(name string) error {
	mg.Deps(mg.F(Build, name))
	for part, params := range lookupParams(name) {
		writeLog(fmt.Sprintf("running challenge:part '%s:%s'\n", name, part))
		args := append([]string{"test", "-w", name, "--bin", binaryLoc(name)}, params...)
		if err := sh.RunV("./maelstrom/maelstrom", args...); err != nil {
			_ = mg.Fatalf(1, "error running '%s:%s': %v", name, part, err)
		}
	}
	return nil
}

// Part runs the given {challenge}, {part}. Use mage ls to see valid targets.
func (Run) Part(challenge, part string) error {
	mg.Deps(mg.F(Build, challenge))
	args := append([]string{"test", "-w", challenge, "--bin", binaryLoc(challenge)}, lookupPartParams(challenge, part)...)
	writeLog(fmt.Sprintf("running challenge:part '%s:%s'\n", challenge, part))
	return sh.RunV("./maelstrom/maelstrom", args...)
}

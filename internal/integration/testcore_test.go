package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// fakeCoreBehavior is deliberately limited to this test package. The copied
// test binary is never installed or selected by production code.
type fakeCoreBehavior struct {
	Version   string `json:"version"`
	CheckExit int    `json:"checkExit"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ArgsFile  string `json:"argsFile"`
}

func TestMain(m *testing.M) {
	if os.Getenv("SINGBOX_WEB_TEST_HELPER") == "1" {
		os.Exit(runFakeCore())
	}
	os.Exit(m.Run())
}

func runFakeCore() int {
	control := os.Getenv("SINGBOX_WEB_TEST_CONTROL")
	contents, err := os.ReadFile(control)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var behavior fakeCoreBehavior
	if err := json.Unmarshal(contents, &behavior); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	args := os.Args[1:]
	if behavior.ArgsFile != "" {
		entry, _ := json.Marshal(args)
		file, err := os.OpenFile(behavior.ArgsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		_, err = file.Write(append(entry, '\n'))
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			fmt.Fprintln(os.Stderr, "无法记录 fake core 参数")
			return 2
		}
	}
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Println(behavior.Version)
		return 0
	case "check", "run":
		if !slicesContainConfigDir(args) {
			fmt.Fprintln(os.Stderr, "缺少 -C 配置目录")
			return 2
		}
		fmt.Fprint(os.Stdout, behavior.Stdout)
		fmt.Fprint(os.Stderr, behavior.Stderr)
		return behavior.CheckExit
	default:
		fmt.Fprintln(os.Stderr, "不支持的 fake core 命令")
		return 2
	}
}

func slicesContainConfigDir(args []string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "-C" && strings.TrimSpace(args[index+1]) != "" {
			return true
		}
	}
	return false
}

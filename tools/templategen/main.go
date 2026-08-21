package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	input := flag.String("input", "", "输入配置")
	profile := flag.String("profile", "", "档案模板输出")
	resources := flag.String("resources", "", "规则资源模板输出")
	bindings := flag.String("bindings", "", "规则绑定模板输出")
	flag.Parse()
	if *input == "" || *profile == "" || *resources == "" || *bindings == "" {
		fmt.Fprintln(os.Stderr, "必须指定 -input -profile -resources -bindings")
		os.Exit(2)
	}
	source, err := os.ReadFile(*input)
	if err == nil {
		var result Result
		result, err = Transform(source)
		if err == nil {
			err = os.WriteFile(*profile, result.Profile, 0o644)
		}
		if err == nil {
			err = os.WriteFile(*resources, result.Resources, 0o644)
		}
		if err == nil {
			err = os.WriteFile(*bindings, result.Bindings, 0o644)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func main() {
	total := flag.Int("total", 1, "total number of shards")
	index := flag.Int("index", 0, "zero-based shard index")
	list := flag.String("list", "", "comma separated packages to shard; defaults to go list ./...")
	file := flag.String("file", "", "optional file containing newline separated packages")
	flag.Parse()

	if *total <= 0 {
		fmt.Fprintln(os.Stderr, "total must be > 0")
		os.Exit(1)
	}
	if *index < 0 || *index >= *total {
		fmt.Fprintf(os.Stderr, "index must be between 0 and %d\n", *total-1)
		os.Exit(1)
	}

	pkgs := collectPackages(*list, *file)
	if len(pkgs) == 0 {
		return
	}

	sort.Strings(pkgs)
	var selected []string
	for i, pkg := range pkgs {
		if i%*total == *index {
			selected = append(selected, pkg)
		}
	}

	for _, pkg := range selected {
		fmt.Println(pkg)
	}
}

func collectPackages(list, file string) []string {
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open list file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		var pkgs []string
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			if pkg := strings.TrimSpace(scanner.Text()); pkg != "" {
				pkgs = append(pkgs, pkg)
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "scan list file: %v\n", err)
			os.Exit(1)
		}
		return pkgs
	}
	if list != "" {
		fields := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == '\n' })
		var pkgs []string
		for _, f := range fields {
			if trimmed := strings.TrimSpace(f); trimmed != "" {
				pkgs = append(pkgs, trimmed)
			}
		}
		return pkgs
	}
	cmd := exec.Command("go", "list", "./...")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go list failed: %v\n", err)
		os.Exit(1)
	}
	lines := strings.Split(string(out), "\n")
	var pkgs []string
	for _, line := range lines {
		if pkg := strings.TrimSpace(line); pkg != "" {
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}

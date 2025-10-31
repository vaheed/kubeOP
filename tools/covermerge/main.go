package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	inputs := flag.Args()
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "no coverage files provided")
		os.Exit(1)
	}

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	headerWritten := false
	for _, path := range inputs {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
			os.Exit(1)
		}
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			line := scanner.Text()
			if lineNum == 0 {
				if !headerWritten {
					fmt.Fprintln(out, line)
					headerWritten = true
				}
			} else {
				fmt.Fprintln(out, line)
			}
			lineNum++
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "scan %s: %v\n", path, err)
			os.Exit(1)
		}
		f.Close()
	}
}

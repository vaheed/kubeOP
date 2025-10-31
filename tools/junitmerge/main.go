package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
)

type testSuites struct {
	XMLName xml.Name    `xml:"testsuites"`
	Suites  []testSuite `xml:"testsuite"`
}

type testSuite struct {
	XMLName    xml.Name      `xml:"testsuite"`
	Name       string        `xml:"name,attr"`
	Tests      int           `xml:"tests,attr"`
	Failures   int           `xml:"failures,attr"`
	Errors     int           `xml:"errors,attr"`
	Skipped    int           `xml:"skipped,attr"`
	Time       float64       `xml:"time,attr"`
	Properties []property    `xml:"properties>property"`
	Cases      []testCase    `xml:"testcase"`
	SystemOut  []cdataString `xml:"system-out"`
	SystemErr  []cdataString `xml:"system-err"`
}

type testCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	Class     string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *cdataString  `xml:"failure"`
	Skipped   *cdataString  `xml:"skipped"`
	SystemOut []cdataString `xml:"system-out"`
	SystemErr []cdataString `xml:"system-err"`
}

type property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type cdataString struct {
	Text string `xml:",innerxml"`
}

func main() {
	flag.Parse()
	inputs := flag.Args()
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "no junit files provided")
		os.Exit(1)
	}

	merged := &testSuites{}
	for _, in := range inputs {
		if err := mergeFile(merged, in); err != nil {
			fmt.Fprintf(os.Stderr, "merge %s: %v\n", in, err)
			os.Exit(1)
		}
	}

	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "  ")
	if err := enc.Encode(merged); err != nil {
		fmt.Fprintf(os.Stderr, "encode junit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
}

func mergeFile(suites *testSuites, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "testsuite":
				var ts testSuite
				if err := dec.DecodeElement(&ts, &se); err != nil {
					return err
				}
				suites.Suites = append(suites.Suites, ts)
			case "testsuites":
				var tss testSuites
				if err := dec.DecodeElement(&tss, &se); err != nil {
					return err
				}
				suites.Suites = append(suites.Suites, tss.Suites...)
			}
		}
	}
	return nil
}

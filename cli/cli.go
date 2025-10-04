package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ryanolee/go-chaff"
)

var (
	version   = "UNKNOWN"
	commit    = "UNKNOWN"
	buildDate = "UNKNOWN"
)

func main() {
	// String Flags
	path := flag.String("file", "", "Specify a file path to read the JSON Schema from")
	output := flag.String("output", "", "Specify file path to write generated output to.")

	// Bool Flags
	formatted := flag.Bool("format", false, "Format JSON output.")
	showHelp := flag.Bool("help", false, "Print out help.")
	verbose := flag.Bool("verbose", false, "Print out detailed error information.")
	showVersion := flag.Bool("version", false, "Print out cli version information.")
	flag.Parse()

	if *showHelp {
		fmt.Println("CLI too for generating random JSON data matching given JSON schema\nUsage: go-chaff [flags]")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("Version: %s\nCommit: %s\nBuilt On: %s\n", version, commit, buildDate)
		os.Exit(0)
	}

	var generator chaff.RootGenerator
	var err error

	if *path != "" {
		generator, err = chaff.ParseSchemaFileWithDefaults(*path)
		checkErr(err)
	} else if hasStdin() {
		stdin := readStdin()
		generator, err = chaff.ParseSchemaWithDefaults(stdin)
		checkErr(err)
	} else {
		checkErr(fmt.Errorf("no schema specified! (On Stdin or through the --file flag)"))
	}

	if *verbose {
		fmt.Printf("Schema compiled successfully to the following generator tree: %s\n", generator)

		if len(generator.Metadata.Errors) != 0 {
			fmt.Println("Passed schema failed to fully compile with the following errors:")
		}

		for key, value := range generator.Metadata.Errors {
			fmt.Printf(" - [%s] %s \n", key, value)
		}
	}

	result := generator.GenerateWithDefaults()
	checkErr(err)

	var res []byte
	if *formatted {
		res, err = json.MarshalIndent(result, "", "    ")
	} else {
		res, err = json.Marshal(result)
	}

	checkErr(err)

	if *output != "" {
		writeFile(res, *output)
	} else {
		fmt.Print(string(res))
	}

}

func writeFile(data []byte, filepath string) {
	_, err := os.Stat(filepath)

	if !errors.Is(err, os.ErrNotExist) {
		checkErr(fmt.Errorf("file '%s' already exists", filepath))
	}

	file, err := os.Create(filepath)

	checkErr(err)
	_, err = file.Write(data)
	checkErr(err)

	err = file.Close()
	checkErr(err)
}

func readStdin() []byte {
	if hasStdin() {
		stdin, err := io.ReadAll(os.Stdin)
		checkErr(err)
		return stdin
	}

	return nil
}

func hasStdin() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ryanolee/go-chaff"
	"github.com/ryanolee/go-chaff/internal/util"
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

	// Fetch flags
	allowedHosts := flag.String("allowed-hosts", "", "Comma separated list of allowed hosts to fetch remote $ref documents from over HTTP(S). If empty http and https resolution will fail.")
	allowInsecure := flag.Bool("allow-insecure", false, "Allow fetching remote $ref documents over insecure HTTP connections.")
	allowOutsideCwd := flag.Bool("allow-outside-cwd", false, "Allow fetching $ref documents from file system paths outside the current working directory.")
	allowedPaths := flag.String("allowed-paths", "", "Comma separated list of allowed file system paths to fetch $ref documents from.")

	// Generator complexity flags
	bypassCyclicReferenceCheck := flag.Bool("bypass-cyclic-reference-check", false, "Bypass cyclic reference check when generating schemas with cyclic $ref references.")
	maximumReferenceDepth := flag.Int("maximum-reference-depth", 10, "Maximum depth of $ref references to resolve at once when generating data.")

	// Performance flags
	MaximumIfAttempts := flag.Int("maximum-if-attempts", 100, "Maximum number of attempts to satisfy 'if' conditions when generating data.")
	MaximumOneOfAttempts := flag.Int("maximum-oneof-attempts", 100, "Maximum number of attempts to satisfy 'oneOf' conditions when generating data.")
	MaximumGenerationSteps := flag.Int("maximum-generation-steps", 1000, "Maximum number of generation steps to perform before reducing the effort put into the generation process to a bare minimum.")
	CutoffGenerationSteps := flag.Int("cutoff-generation-steps", 2000, "Maximum number of generation steps to perform before aborting generation entirely and returning what was generated.")

	// Bool Flags
	formatted := flag.Bool("format", false, "Format JSON output.")
	showHelp := flag.Bool("help", false, "Print out help.")
	verbose := flag.Bool("verbose", false, "Print out detailed error information.")
	showVersion := flag.Bool("version", false, "Print out cli version information.")
	flag.Parse()

	if *showHelp {
		fmt.Println("CLI tool for generating random JSON data matching given JSON schema\nUsage: go-chaff [flags]")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("Version: %s\nCommit: %s\nBuilt On: %s\n", version, commit, buildDate)
		os.Exit(0)
	}

	var generator chaff.RootGenerator
	var err error

	parserOptions := &chaff.ParserOptions{
		DocumentFetchOptions: chaff.DocumentFetchOptions{
			HTTPFetchOptions:       getHttpDocumentFetcherOptionsFromFlags(allowedHosts, allowInsecure),
			FileSystemFetchOptions: getFileSystemDocumentFetcherOptionsFromFlags(allowOutsideCwd, allowedPaths),
		},
	}

	if *path != "" {
		generator, err = chaff.ParseSchemaFile(*path, parserOptions)

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

		if generator.Metadata.Errors.HasErrors() {
			fmt.Println("Passed schema failed to fully compile with the following errors:")
		}

		for key, value := range generator.Metadata.Errors.CollectErrors() {
			fmt.Printf(" - [%s] %s \n", key, value)
		}
	}

	generatorOptions := &chaff.GeneratorOptions{
		BypassCyclicReferenceCheck: *bypassCyclicReferenceCheck,
		MaximumReferenceDepth:      *maximumReferenceDepth,
		MaximumIfAttempts:          *MaximumIfAttempts,
		MaximumOneOfAttempts:       *MaximumOneOfAttempts,
		MaximumGenerationSteps:     *MaximumGenerationSteps,
		CutoffGenerationSteps:      *CutoffGenerationSteps,
	}

	result := generator.Generate(generatorOptions)
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

func getHttpDocumentFetcherOptionsFromFlags(allowedHosts *string, allowInsecure *bool) chaff.HTTPFetchOptions {
	if (allowedHosts != nil && *allowedHosts == "") && (allowInsecure == nil || !*allowInsecure) {
		return chaff.HTTPFetchOptions{}
	}

	return chaff.HTTPFetchOptions{
		Enabled:       true,
		AllowedHosts:  parseCommaSeparatedList(allowedHosts),
		AllowInsecure: util.GetZeroIfNil(allowInsecure, false),
	}
}

func getFileSystemDocumentFetcherOptionsFromFlags(allowOutsideCwd *bool, allowedPaths *string) chaff.FileSystemFetchOptions {
	if (allowOutsideCwd == nil || !*allowOutsideCwd) && (allowedPaths == nil || *allowedPaths == "") {
		return chaff.FileSystemFetchOptions{}
	}

	return chaff.FileSystemFetchOptions{
		Enabled:         true,
		AllowOutsideCwd: util.GetZeroIfNil(allowOutsideCwd, false),
		AllowedPaths:    parseCommaSeparatedList(allowedPaths),
	}
}

func parseCommaSeparatedList(input *string) []string {
	if input == nil || *input == "" {
		return []string{}
	}

	return strings.Split(*input, ",")
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

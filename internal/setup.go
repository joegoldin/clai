package internal

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/baalimago/go_away_boilerplate/pkg/ancli"
	"github.com/baalimago/go_away_boilerplate/pkg/misc"
)

type PromptConfig struct {
	Photo string `yaml:"photo"`
	Query string `yaml:"query"`
}

func returnNonDefault[T comparable](a, b, defaultVal T) (T, error) {
	if a != defaultVal && b != defaultVal {
		return defaultVal, fmt.Errorf("values are mutually exclusive")
	}
	if a != defaultVal {
		return a, nil
	}
	if b != defaultVal {
		return b, nil
	}
	return defaultVal, nil
}

var defaultFlags = flagSet{
	chatModel:     "gpt-4-turbo-preview",
	photoModel:    "dall-e-3",
	picturePrefix: "clai",
	pictureDir:    fmt.Sprintf("%v/Pictures", os.Getenv("HOME")),
	stdinReplace:  "",
	printRaw:      false,
	replyMode:     false,
}

func Setup(usage string) (string, ChatModelQuerier, PhotoQuerier, []string) {
	flagSet := setupFlags(defaultFlags)
	API_KEY := os.Getenv("OPENAI_API_KEY")
	if API_KEY == "" {
		ancli.PrintErr("OPENAI_API_KEY environment variable not set\n")
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		ancli.PrintErr(fmt.Sprintf("failed to get home dir: %v\n", err))
	}

	client := http.Client{}
	cmq := ChatModelQuerier{
		SystemPrompt: "You are an assistent for a CLI interface. Answer concisely and informatively. Prefer markdown if possible.",
		Raw:          flagSet.printRaw,
		Url:          "https://api.openai.com/v1/chat/completions",
		ReplyMode:    flagSet.replyMode,
		home:         home,
		client:       &client,
	}
	pq := PhotoQuerier{
		PhotoDir:     flagSet.pictureDir,
		PhotoPrefix:  flagSet.picturePrefix,
		PromptFormat: "I NEED to test how the tool works with extremely simple prompts. DO NOT add any detail, just use it AS-IS: '%v'",
		url:          "https://api.openai.com/v1/images/generations",
		raw:          flagSet.printRaw,
		client:       &client,
	}

	homedirConfig(&cmq, &pq)
	// Flag overrides homedirConfig
	if flagSet.chatModel != defaultFlags.chatModel {
		cmq.Model = flagSet.chatModel
	}
	if flagSet.printRaw {
		cmq.Raw = true
	}
	if flagSet.photoModel != defaultFlags.photoModel {
		pq.Model = flagSet.photoModel
	}
	if flagSet.picturePrefix != defaultFlags.picturePrefix {
		pq.PhotoPrefix = flagSet.picturePrefix
	}
	if flagSet.pictureDir != defaultFlags.pictureDir {
		pq.PhotoDir = flagSet.pictureDir
	}
	if misc.Truthy(os.Getenv("DEBUG")) {
		ancli.PrintOK(fmt.Sprintf("chatModel: %v\n", cmq))
	}
	return API_KEY, cmq, pq, parseArgsStdin(flagSet.stdinReplace, usage)
}

func exitWithFlagError(err error, shortFlag, longflag string) {
	if err != nil {
		// Im just too lazy to setup the err struct
		if err.Error() == "values are mutually exclusive" {
			ancli.PrintErr(fmt.Sprintf("flags: '%v' and '%v' are mutually exclusive, err: %v\n", shortFlag, longflag, err))
		} else {
			ancli.PrintErr(fmt.Sprintf("unexpected error: %v", err))
		}
		os.Exit(1)
	}
}

func parseArgsStdin(stdinReplace, usage string) []string {
	if misc.Truthy(os.Getenv("DEBUG")) {
		ancli.PrintOK(fmt.Sprintf("stdinReplace: %v\n", stdinReplace))
	}
	args := flag.Args()
	fi, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}
	var hasPipe bool
	if fi.Mode()&os.ModeNamedPipe == 0 {
		hasPipe = false
	} else {
		hasPipe = true
	}
	if len(args) == 1 && !hasPipe {
		if args[0] == "h" || args[0] == "help" || args[0] == "-help" || args[0] == "-h" {
			fmt.Print(usage)
			os.Exit(0)
		}

		if args[0] == "v" || args[0] == "version" || args[0] == "-v" || args[0] == "-version" {
			bi, ok := debug.ReadBuildInfo()
			if !ok {
				ancli.PrintErr("failed to read build info")
				os.Exit(1)
			}
			fmt.Printf("version: %v, go version: %v, checksum: %v\n", bi.Main.Version, bi.GoVersion, bi.Main.Sum)
			os.Exit(0)
		}

		ancli.PrintErr("found no prompt, set args or pipe in some string\n")
		fmt.Print(usage)
		os.Exit(1)
	}
	// If no data is in stdin, simply return args
	if !hasPipe {
		return args
	}

	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		ancli.PrintErr(fmt.Sprintf("failed to read stdin: %v", err))
		os.Exit(1)
	}
	// There is data to read from stdin, so read it
	if err != nil {
		ancli.PrintErr("failed to read from stdin\n")
		os.Exit(1)
	}
	pipeIn := string(inputData)
	if len(args) == 1 {
		args = append(args, strings.Split(pipeIn, " ")...)
	}

	// Replace all occurrence of stdinReplaceSignal with pipeIn
	for i, arg := range args {
		if strings.Contains(arg, stdinReplace) {
			args[i] = strings.ReplaceAll(arg, stdinReplace, pipeIn)
		}
	}

	if misc.Truthy(os.Getenv("DEBUG")) {
		ancli.PrintOK(fmt.Sprintf("args: %v\n", args))
	}
	return args
}

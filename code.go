package code

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/gommon/log"
	"github.com/maddyonline/glot-code-runner/runlib"
	"path/filepath"
)

type File struct {
	Name    string
	Content string
	Id      string `json:"id"`
	Sha     string `json:"sha"`
}

type Input struct {
	Language string
	Files    []File
}

type Output struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"err"`
}

type Runner struct {
	RunnerBinary string
}

type langConfig struct {
	DockerImage string
	IsSupported bool
}

var languages = map[string]*langConfig{
	"assembly":     &langConfig{"", false},
	"bash":         &langConfig{"", false},
	"c":            &langConfig{"", false},
	"clojure":      &langConfig{"", false},
	"coffeescript": &langConfig{"", false},
	"csharp":       &langConfig{"", false},
	"d":            &langConfig{"", false},
	"elixir":       &langConfig{"", false},
	"cpp":          &langConfig{"glot/clang", true},
	"erlang":       &langConfig{"", false},
	"fsharp":       &langConfig{"", false},
	"haskell":      &langConfig{"", false},
	"idris":        &langConfig{"", false},
	"go":           &langConfig{"glot/golang", true},
	"java":         &langConfig{"glot/java", false},
	"javascript":   &langConfig{"glot/javascript", true},
	"julia":        &langConfig{"", false},
	"lua":          &langConfig{"", false},
	"nim":          &langConfig{"", false},
	"ocaml":        &langConfig{"", false},
	"perl":         &langConfig{"", false},
	"php":          &langConfig{"", false},
	"python":       &langConfig{"glot/python", true},
	"ruby":         &langConfig{"", false},
	"rust":         &langConfig{"", false},
	"scala":        &langConfig{"", false},
	"swift":        &langConfig{"", false},
}

func IsNotSupported(lang string) bool {
	return languages[lang] == nil || !languages[lang].IsSupported || languages[lang].DockerImage == ""
}

func NewRunner(pathToRunner string) *Runner {
	dir, _ := filepath.Abs(pathToRunner)
	return &Runner{RunnerBinary: dir}
}

func (r *Runner) RunLocal(input *Input) (*Output, error) {
	log.Info("Starting local run...")
	if IsNotSupported(input.Language) {
		return &Output{}, errors.New(fmt.Sprintf("Language %s not supported", input.Language))
	}
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	inputStream := bytes.NewReader([]byte(inputBytes))
	outputStream := &bytes.Buffer{}

	runlib.Run(inputStream, outputStream)

	output := &Output{}
	err = json.Unmarshal(outputStream.Bytes(), &output)
	if err != nil {
		return nil, err
	}
	return output, nil

}

func (r *Runner) Run(input *Input) (*Output, error) {
	return r.RunLocal(input)
}

func StdinFile(content string) File {
	return File{
		Name:    "_stdin_",
		Content: content,
	}
}

func UpdateStdin(input *Input, stdinFile File) {
	for i, file := range input.Files {
		if file.Name == "_stdin_" {
			input.Files[i].Content = stdinFile.Content
			return
		}
	}
	input.Files = append(input.Files, stdinFile)
	return
}

func MakeInput(language, name, content string, input File) *Input {
	return &Input{
		Language: language,
		Files: []File{
			File{
				Name:    name,
				Content: content,
			},
			input,
		},
	}
}

type Result struct {
	Correct bool
}

func Evaluate(inputGen, inputCode1, inputCode2 *Input, runner *Runner) *Result {
	gen := []chan *Output{
		make(chan *Output),
		make(chan *Output),
	}
	results := make(chan struct {
		InputStr *string
		Output   *Output
	}, 2)
	go func() {
		output, err := runner.Run(inputGen)
		process(output, err)
		go func() { gen[0] <- output }()
		go func() { gen[1] <- output }()
	}()
	inputs := []*Input{inputCode1, inputCode2}
	for i := 0; i < 2; i++ {
		go func(i int) {
			genOutput := <-gen[i]
			input := inputs[i]
			log.Info("Using index: %d", i)
			log.Info("Code: %q", input.Files[0].Content)
			log.Info("Want: %q", genOutput.Stdout)
			log.Info("Before: %#v", input)
			UpdateStdin(input, StdinFile(genOutput.Stdout))
			log.Info("After: %#v", input)
			output, err := runner.Run(input)
			process(output, err)
			results <- struct {
				InputStr *string
				Output   *Output
			}{&genOutput.Stdout, output}
		}(i)
	}
	out1, out2 := <-results, <-results

	if diff(out1.Output.Stdout, out2.Output.Stdout) {
		log.Info("Different on input %q: %q %q", *out1.InputStr, out1.Output.Stdout, out2.Output.Stdout)
		return &Result{false}
	} else {
		log.Info("Identical on input %q", *out1.InputStr)
		return &Result{true}
	}
}

func diff(a, b string) bool {
	return a != b
}

func process(output *Output, err error) {
	if err != nil {
		log.Info("%v", err)
	} else {
		log.Info("%#v", output)
	}
}

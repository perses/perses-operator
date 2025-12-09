// Copyright 2023 The Perses Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/goreleaser/goreleaser/v2/pkg/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const goreleaserFile = ".goreleaser.yaml"

func FromStdout() ([]byte, error) {
	// from https://flaviocopes.com/go-shell-pipes/
	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("the command is intended to work with pipes")
	}

	reader := bufio.NewReader(os.Stdin)
	var output []byte

	for {
		input, readErr := reader.ReadByte()
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, readErr
		}
		output = append(output, input)
	}
	return output, nil
}

func readAndDetect(file string) (data []byte, isJSON bool, err error) {
	if file == "-" {
		data, err = FromStdout()
	} else {
		data, err = os.ReadFile(file) //nolint
	}

	if err != nil {
		return
	}

	// detecting file format
	isJSON = json.Unmarshal(data, &json.RawMessage{}) == nil
	return
}

func Unmarshal(file string, obj any) error {
	data, isJSON, err := readAndDetect(file)
	if err != nil {
		return err
	}
	if isJSON {
		if jsonErr := json.Unmarshal(data, obj); jsonErr != nil {
			return jsonErr
		}
	} else {
		if yamlErr := yaml.Unmarshal(data, obj); yamlErr != nil {
			return yamlErr
		}
	}
	return nil
}

func getCurrentBranch() string {
	branch, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		logrus.WithError(err).Fatal("unable to get the current branch")
	}
	return strings.TrimSpace(string(branch))
}

func main() {
	if getCurrentBranch() != "main" {
		logrus.Warning("script has been executed on a branch different than the main branch")
		return
	}
	goreleaserConfig := &config.Project{}
	if err := Unmarshal(goreleaserFile, goreleaserConfig); err != nil {
		logrus.WithError(err).Fatal("unable to load the goreleaser config")
	}
	for _, dockerConfig := range goreleaserConfig.Dockers {
		for _, image := range dockerConfig.ImageTemplates {
			if output, err := exec.Command("docker", "push", image).Output(); err != nil { //nolint: gosec
				logrus.WithError(err).Fatalf("unable to push the docker image %q. Output: %q", image, output)
			}
		}
	}
	for _, manifestConfig := range goreleaserConfig.DockerManifests {
		args := []string{"manifest", "create", manifestConfig.NameTemplate}
		args = append(args, manifestConfig.ImageTemplates...)
		if output, err := exec.Command("docker", args...).Output(); err != nil { //nolint: gosec
			logrus.WithError(err).Fatalf("unable to create the docker manifest %q. Output: %q", manifestConfig.NameTemplate, output)
		}
		if output, err := exec.Command("docker", "manifest", "push", manifestConfig.NameTemplate).Output(); err != nil { //nolint:gosec
			logrus.WithError(err).Fatalf("unable to push the docker manifest %q. Output: %q", manifestConfig.NameTemplate, output)
		}
	}
}

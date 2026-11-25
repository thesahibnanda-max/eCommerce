package utils

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/go-faster/yaml"
)

var (
	json          = sonic.ConfigFastest
	envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
)

func FromStringToObjectJSON[T any](input string) (result T, err error) {
	defer recoverAsError(&result, &err)
	err = json.Unmarshal([]byte(input), &result)
	return result, err
}

func FromStringToObjectYAML[T any](input string) (result T, err error) {
	defer recoverAsError(&result, &err)
	err = yaml.Unmarshal([]byte(input), &result)
	return result, err
}

func FromObjectToStringJSON[T any](input T) (result string, err error) {
	defer recoverAsError(&result, &err)
	return json.MarshalToString(input)
}

func FromObjectToStringYAML[T any](input T) (result string, err error) {
	defer recoverAsError(&result, &err)
	byteYAML, err := yaml.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(byteYAML), nil
}

func ReadParametrisedYAML[T any](input string) (result T, err error) {
	defer recoverAsError(&result, &err)
	var root yaml.Node
	if err = yaml.Unmarshal([]byte(input), &root); err != nil {
		return result, err
	}
	if root.Kind == 0 {
		return result, nil
	}
	var missing []string
	expandEnvVars(&root, &missing)
	if len(missing) > 0 {
		return result, fmt.Errorf("%w: %s", ErrEnvVarMissing, strings.Join(missing, ", "))
	}
	err = root.Decode(&result)
	return result, err
}

func expandEnvVars(node *yaml.Node, missing *[]string) {
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "${") {
		expanded := envVarPattern.ReplaceAllStringFunc(node.Value, func(match string) string {
			name := match[2 : len(match)-1]
			value, ok := os.LookupEnv(name)
			if !ok && !slices.Contains(*missing, name) {
				*missing = append(*missing, name)
			}
			return value
		})
		if expanded != node.Value {
			node.Value = expanded
			node.Tag = ""
		}
	}
	for _, child := range node.Content {
		expandEnvVars(child, missing)
	}
}

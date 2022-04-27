package lagoon

import (
	"fmt"
)

// EnvironmentVariable is used to define Lagoon environment variables.
type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope"`
}

// MergeVariables merges lagoon environment variables.
func MergeVariables(project, environment []EnvironmentVariable) []EnvironmentVariable {
	allVars := []EnvironmentVariable{}
	existsInEnvironment := false
	for _, pVar := range project {
		add := EnvironmentVariable{}
		for _, eVar := range environment {
			if pVar.Name == eVar.Name {
				existsInEnvironment = true
				add = eVar
			}
		}
		if existsInEnvironment {
			allVars = append(allVars, add)
			existsInEnvironment = false
		} else {
			allVars = append(allVars, pVar)
		}
	}
	return allVars
}

// GetLagoonVariable returns a given environment variable
func GetLagoonVariable(name string, variables []EnvironmentVariable) (*EnvironmentVariable, error) {
	for _, v := range variables {
		if v.Name == name {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("variable not found")
}

// VariableExists checks if a variable exists in a slice of environment variables
func VariableExists(vars *[]EnvironmentVariable, name, value string) bool {
	exists := false
	for _, v := range *vars {
		if v.Name == name && v.Value == value {
			exists = true
		}
	}
	return exists
}
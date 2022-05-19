package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/uselagoon/build-deploy-tool/internal/helpers"
	"github.com/uselagoon/build-deploy-tool/internal/lagoon"
	"sigs.k8s.io/yaml"
)

var lagoonYml, environmentName, projectName, activeEnvironment, standbyEnvironment, environmentType string
var buildType, lagoonVersion, branch, prTitle, prNumber, prHeadBranch, prBaseBranch string
var projectVariables, environmentVariables, monitoringStatusPageID, monitoringContact string
var templateValues, savedTemplates, fastlyCacheNoCahce, fastlyServiceID, fastlyAPISecretPrefix string
var monitoringEnabled bool

// collectBuildValues is used to collect variables and values that are used within a build
func collectBuildValues(debug bool, activeEnv, standbyEnv *bool,
	lagoonEnvVars *[]lagoon.EnvironmentVariable,
	lagoonValues *lagoon.BuildValues,
	lYAML *lagoon.YAML,
	autogenRoutes *lagoon.RoutesV2,
	mainRoutes *lagoon.RoutesV2,
	activeStandbyRoutes *lagoon.RoutesV2,
) error {
	var err error
	// environment variables will override what is provided by flags
	// the following variables have been identified as used by custom-ingress objects
	// these are available within a lagoon build as standard
	monitoringContact = helpers.GetEnv("MONITORING_ALERTCONTACT", monitoringContact, debug)
	monitoringStatusPageID = helpers.GetEnv("MONITORING_STATUSPAGEID", monitoringStatusPageID, debug)
	projectName = helpers.GetEnv("PROJECT", projectName, debug)
	environmentName = helpers.GetEnv("ENVIRONMENT", environmentName, debug)
	branch = helpers.GetEnv("BRANCH", branch, debug)
	prNumber = helpers.GetEnv("PR_NUMBER", prNumber, debug)
	prTitle = helpers.GetEnv("PR_NUMBER", prTitle, debug)
	prHeadBranch = helpers.GetEnv("PR_HEAD_BRANCH", prHeadBranch, debug)
	prBaseBranch = helpers.GetEnv("PR_BASE_BRANCH", prBaseBranch, debug)
	environmentType = helpers.GetEnv("ENVIRONMENT_TYPE", environmentType, debug)
	buildType = helpers.GetEnv("BUILD_TYPE", buildType, debug)
	activeEnvironment = helpers.GetEnv("ACTIVE_ENVIRONMENT", activeEnvironment, debug)
	standbyEnvironment = helpers.GetEnv("STANDBY_ENVIRONMENT", standbyEnvironment, debug)
	fastlyCacheNoCahce = helpers.GetEnv("LAGOON_FASTLY_NOCACHE_SERVICE_ID", fastlyCacheNoCahce, debug)
	lagoonVersion = helpers.GetEnv("LAGOON_VERSION", lagoonVersion, debug)

	// read the .lagoon.yml file
	lPolysite := make(map[string]interface{})
	if err := lagoon.UnmarshalLagoonYAML(lagoonYml, lYAML, &lPolysite); err != nil {
		return fmt.Errorf("couldn't read file %v: %v", lagoonYml, err)
	}

	// if this is a polysite, then unmarshal the polysite data into a normal lagoon environments yaml
	if _, ok := lPolysite[projectName]; ok {
		s, _ := yaml.Marshal(lPolysite[projectName])
		_ = yaml.Unmarshal(s, &lYAML)
	}

	lCompose := lagoon.Compose{}
	// unmarshal the docker-compose.yml file
	if err = lagoon.UnmarshaDockerComposeYAML(lYAML.DockerComposeYAML, &lCompose); err != nil {
		return err
	}

	lagoonValues.Project = projectName
	lagoonValues.Environment = environmentName
	lagoonValues.EnvironmentType = environmentType
	lagoonValues.BuildType = buildType
	lagoonValues.LagoonVersion = lagoonVersion
	lagoonValues.ActiveEnvironment = activeEnvironment
	lagoonValues.StandbyEnvironment = standbyEnvironment
	switch buildType {
	case "branch", "promote":
		lagoonValues.Branch = branch
	case "pullrequest":
		lagoonValues.PRNumber = prNumber
		lagoonValues.PRTitle = prTitle
		lagoonValues.PRHeadBranch = prHeadBranch
		lagoonValues.PRBaseBranch = prBaseBranch
	}

	if projectName == "" || environmentName == "" || environmentType == "" || buildType == "" {
		return fmt.Errorf("Missing arguments: project-name, environment-name, environment-type, or build-type not defined")
	}
	switch buildType {
	case "branch", "promote":
		if branch == "" {
			return fmt.Errorf("Missing arguments: branch not defined")
		}
	case "pullrequest":
		if prNumber == "" || prHeadBranch == "" || prBaseBranch == "" {
			return fmt.Errorf("Missing arguments: pullrequest-number, pullrequest-head-branch, or pullrequest-base-branch not defined")
		}
	}

	// get the project and environment variables
	projectVariables = helpers.GetEnv("LAGOON_PROJECT_VARIABLES", projectVariables, debug)
	environmentVariables = helpers.GetEnv("LAGOON_ENVIRONMENT_VARIABLES", environmentVariables, debug)

	// by default, environment routes are not monitored
	monitoringEnabled = false
	if environmentType == "production" {
		// if this is a production environment, monitoring IS enabled
		monitoringEnabled = true
		// check if the environment is active or standby
		if environmentName == activeEnvironment {
			*activeEnv = true
		}
		if environmentName == standbyEnvironment {
			*standbyEnv = true
		}
	}

	// unmarshal and then merge the two so there is only 1 set of variables to iterate over
	projectVars := []lagoon.EnvironmentVariable{}
	envVars := []lagoon.EnvironmentVariable{}
	json.Unmarshal([]byte(projectVariables), &projectVars)
	json.Unmarshal([]byte(environmentVariables), &envVars)
	mergedVariables := lagoon.MergeVariables(projectVars, envVars)

	// create the services map
	lagoonValues.Services = make(map[string]lagoon.ServiceValues)
	lagoonServiceTypes, _ := lagoon.GetLagoonVariable("LAGOON_SERVICE_TYPES", []string{"build"}, mergedVariables)
	// convert docker-compose services to servicevalues
	for csName, csValues := range lCompose.Services {
		cService, err := composeToServiceValues(lYAML, lagoonValues, lagoonServiceTypes, csName, csValues)
		if err != nil {
			return err
		}
		lagoonValues.Services[csName] = cService
	}

	// create all the routes for this environment and store the primary and secondary routes into values
	// populate the autogenRoutes, mainRoutes and activeStandbyRoutes here and load them
	lagoonValues.Route, lagoonValues.Routes, lagoonValues.AutogeneratedRoutes, err = generateRoutes(
		mergedVariables,
		*lagoonValues,
		*lYAML,
		autogenRoutes,
		mainRoutes,
		activeStandbyRoutes,
		*activeEnv,
		*standbyEnv,
		debug,
	)
	if err != nil {
		return err
	}

	// collect a bunch of the default LAGOON_X based build variables that are injected into `lagoon-env` and make them available
	configVars := collectBuildVariables(*lagoonValues)
	// add the calculated build runtime variables into the existing variable slice
	// this will later be used to add `runtime|global` scope into the `lagoon-env` configmap
	*lagoonEnvVars = lagoon.MergeVariables(mergedVariables, configVars)
	return nil
}

func composeToServiceValues(lYAML *lagoon.YAML, lagoonValues *lagoon.BuildValues, lagoonServiceTypes *lagoon.EnvironmentVariable, csName string, csValues lagoon.Service) (lagoon.ServiceValues, error) {
	lagoonType := lagoon.CheckServiceLagoonLabel(csValues.Labels, "lagoon.type")
	autogenEnabled := true
	// check if autogenerated routes are disabled
	if lYAML.Routes.Autogenerate.Enabled != nil {
		if *lYAML.Routes.Autogenerate.Enabled == false {
			autogenEnabled = false
		}
	}
	// check if pullrequests autogenerated routes are disabled
	if lagoonValues.BuildType == "pullrequest" && lYAML.Routes.Autogenerate.AllowPullRequests != nil {
		if *lYAML.Routes.Autogenerate.AllowPullRequests == false {
			autogenEnabled = false
		} else {
			autogenEnabled = true
		}
	}
	// check if this environment has autogenerated routes disabled
	if lYAML.Environments[lagoonValues.Branch].AutogenerateRoutes != nil {
		if *lYAML.Environments[lagoonValues.Branch].AutogenerateRoutes == false {
			autogenEnabled = false
		} else {
			autogenEnabled = true
		}
	}
	if lagoonType != "" {
		if value, ok := lYAML.Environments[environmentName].Types[csName]; ok {
			lagoonType = value
		}
		if lagoonServiceTypes != nil {
			serviceTypesSplit := strings.Split(lagoonServiceTypes.Value, ",")
			for _, sType := range serviceTypesSplit {
				sTypeSplit := strings.Split(sType, ":")
				if sTypeSplit[0] == csName {
					lagoonType = sTypeSplit[1]
				}
			}
		}
		serviceAutogenerated := lagoon.CheckServiceLagoonLabel(csValues.Labels, "lagoon.autogeneratedroute")
		if serviceAutogenerated != "" {
			if reflect.TypeOf(serviceAutogenerated).Kind() == reflect.String {
				vBool, err := strconv.ParseBool(serviceAutogenerated)
				if err == nil {
					autogenEnabled = vBool
				}
			}
		}
		lagoonTypeName := lagoon.CheckServiceLagoonLabel(csValues.Labels, "lagoon.name")
		if lagoonTypeName != "" {
			if lagoonTypeName != csName {
				// if there is a lagoon.name label on a service, then this service does not need an autogenerated route
				// as it is likely an ancillary pod for another service (the one in the `lagoon.name` label :))
				autogenEnabled = false
			}
		}
		cService := lagoon.ServiceValues{
			Name:                csName,
			Type:                lagoonType,
			AutogeneratedRoutes: autogenEnabled,
		}
		return cService, nil
	}
	return lagoon.ServiceValues{}, fmt.Errorf("Service %s has no `lagoon.type` label in the docker-compose.yml file", csName)
}

func collectBuildVariables(lagoonValues lagoon.BuildValues) []lagoon.EnvironmentVariable {
	vars := []lagoon.EnvironmentVariable{}
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_PROJECT", Value: lagoonValues.Project, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_ENVIRONMENT", Value: lagoonValues.Project, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_ENVIRONMENT_TYPE", Value: lagoonValues.Project, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_GIT_SHA", Value: lagoonValues.Project, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_KUBERNETES", Value: lagoonValues.Project, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_GIT_SAFE_BRANCH", Value: lagoonValues.Project, Scope: "runtime"}) //deprecated???
	if lagoonValues.BuildType == "branch" {
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_GIT_BRANCH", Value: lagoonValues.Branch, Scope: "runtime"})
	}
	if lagoonValues.BuildType == "pullrequest" {
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_PR_HEAD_BRANCH", Value: lagoonValues.PRHeadBranch, Scope: "runtime"})
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_PR_BASE_BRANCH", Value: lagoonValues.PRBaseBranch, Scope: "runtime"})
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_PR_TITLE", Value: lagoonValues.PRTitle, Scope: "runtime"})
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_PR_NUMBER", Value: lagoonValues.PRNumber, Scope: "runtime"})
	}
	if lagoonValues.ActiveEnvironment != "" {
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_ACTIVE_ENVIRONMENT", Value: lagoonValues.ActiveEnvironment, Scope: "runtime"})
	}
	if lagoonValues.StandbyEnvironment != "" {
		vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_STANDBY_ENVIRONMENT", Value: lagoonValues.StandbyEnvironment, Scope: "runtime"})
	}
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_ROUTE", Value: lagoonValues.Route, Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_ROUTES", Value: strings.Join(lagoonValues.Routes, ","), Scope: "runtime"})
	vars = append(vars, lagoon.EnvironmentVariable{Name: "LAGOON_AUTOGENERATED_ROUTES", Value: strings.Join(lagoonValues.AutogeneratedRoutes, ","), Scope: "runtime"})
	return vars
}

func unsetEnvVars(localVars []struct {
	name  string
	value string
}) {
	varNames := []string{"MONITORING_ALERTCONTACT", "MONITORING_STATUSPAGEID",
		"PROJECT", "ENVIRONMENT", "BRANCH", "PR_NUMBER", "PR_HEAD_BRANCH",
		"PR_BASE_BRANCH", "ENVIRONMENT_TYPE", "BUILD_TYPE", "ACTIVE_ENVIRONMENT",
		"STANDBY_ENVIRONMENT", "LAGOON_FASTLY_NOCACHE_SERVICE_ID", "LAGOON_PROJECT_VARIABLES",
		"LAGOON_ENVIRONMENT_VARIABLES", "LAGOON_VERSION",
	}
	for _, varName := range varNames {
		os.Unsetenv(varName)
	}
	for _, varName := range localVars {
		os.Unsetenv(varName.name)
	}
}

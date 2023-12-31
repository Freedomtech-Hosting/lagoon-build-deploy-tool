package generator

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	composetypes "github.com/compose-spec/compose-go/types"
	"github.com/uselagoon/build-deploy-tool/internal/helpers"
	"github.com/uselagoon/build-deploy-tool/internal/lagoon"
)

// this is a map that maps old service types to their new service types
var oldServiceMap = map[string]string{
	"mariadb-shared":        "mariadb-dbaas",
	"postgres-shared":       "postgres-dbaas",
	"mongo-shared":          "mongodb-dbaas",
	"python-ckandatapusher": "python",
}

// these are lagoon types that support autogenerated routes
var supportedAutogeneratedTypes = []string{
	// "kibana", //@TODO: don't even need this anymore?
	"basic",
	"basic-persistent",
	"bitcoind",
	"node",
	"node-persistent",
	"nginx",
	"nginx-php",
	"nginx-php-persistent",
	"varnish",
	"varnish-persistent",
	"python-persistent",
	"python",
}

// just some default values for services
var defaultServiceValues = map[string]map[string]string{
	"elasticsearch": map[string]string{
		"persistentPath": "/usr/share/elasticsearch/data",
		"persistentSize": "5Gi",
	},
	"opensearch": map[string]string{
		"persistentPath": "/usr/share/opensearch/data",
		"persistentSize": "5Gi",
	},
	"mariadb-single": map[string]string{
		"persistentPath": "/var/lib/mysql",
		"persistentSize": "5Gi",
	},
	"postgres-single": map[string]string{
		"persistentPath": "/var/lib/postgresql/data",
		"persistentSize": "5Gi",
	},
	"mongodb-single": map[string]string{
		"persistentPath": "/data/db",
		"persistentSize": "5Gi",
	},
	"varnish-persistent": map[string]string{
		"persistentPath": "/var/cache/varnish",
		"persistentSize": "5Gi",
	},
	"rabbitmq": map[string]string{
		"persistentPath": "/var/lib/rabbitmq",
		"persistentSize": "5Gi",
	},
	"redis-persistent": map[string]string{
		"persistentPath": "/data",
		"persistentSize": "5Gi",
	},
}

// generateServicesFromDockerCompose unmarshals the docker-compose file and processes the services using composeToServiceValues
func generateServicesFromDockerCompose(
	buildValues *BuildValues,
	lYAML *lagoon.YAML,
	lagoonEnvVars []lagoon.EnvironmentVariable,
	ignoreNonStringKeyErrors, ignoreMissingEnvFiles, debug bool,
) error {
	// take lagoon envvars and create new map for being unmarshalled against the docker-compose file
	composeVars := make(map[string]string)
	for _, envvar := range lagoonEnvVars {
		composeVars[envvar.Name] = envvar.Value
	}

	// create the services map
	buildValues.Services = []ServiceValues{}

	// unmarshal the docker-compose.yml file
	lCompose, lComposeOrder, err := lagoon.UnmarshaDockerComposeYAML(lYAML.DockerComposeYAML, ignoreNonStringKeyErrors, ignoreMissingEnvFiles, composeVars)
	if err != nil {
		return err
	}

	// convert docker-compose services to servicevalues,
	// range over the original order of the docker-compose file when setting services
	for _, service := range lComposeOrder {
		for _, composeServiceValues := range lCompose.Services {
			if service.Name == composeServiceValues.Name {
				cService, err := composeToServiceValues(buildValues, lYAML, composeServiceValues.Name, composeServiceValues, debug)
				if err != nil {
					return err
				}
				buildValues.Services = append(buildValues.Services, cService)
			}
		}
	}
	return nil
}

// composeToServiceValues is the primary function used to pre-seed how templates are created
// it reads the docker-compose file and converts each service into a ServiceValues struct
// this is the "known state" of that service, and all subsequent steps to create templates will use this data unmodified
func composeToServiceValues(
	buildValues *BuildValues,
	lYAML *lagoon.YAML,
	composeService string,
	composeServiceValues composetypes.ServiceConfig,
	debug bool,
) (ServiceValues, error) {
	lagoonType := ""
	// if there are no labels, then this is probably not going to end up in Lagoon
	// the lagoonType check will skip to the end and return an empty service definition
	if composeServiceValues.Labels != nil {
		lagoonType = lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.type")
	}
	if lagoonType == "" {
		return ServiceValues{}, fmt.Errorf("No lagoon.type has been set for service %s. If a Lagoon service is not required, please set the lagoon.type to 'none' for this service in docker-compose.yaml. See the Lagoon documentation for supported service types.", composeService)
	} else {
		// if the lagoontype is populated, even none is valid as there may be a servicetype override in an environment variable
		autogenEnabled := true
		autogenTLSAcmeEnabled := true
		// check if autogenerated routes are disabled
		if lYAML.Routes.Autogenerate.Enabled != nil {
			if *lYAML.Routes.Autogenerate.Enabled == false {
				autogenEnabled = false
			}
		}
		// check if pullrequests autogenerated routes are disabled
		if buildValues.BuildType == "pullrequest" && lYAML.Routes.Autogenerate.AllowPullRequests != nil {
			if *lYAML.Routes.Autogenerate.AllowPullRequests == false {
				autogenEnabled = false
			} else {
				autogenEnabled = true
			}
		}
		// check if this environment has autogenerated routes disabled
		if lYAML.Environments[buildValues.Branch].AutogenerateRoutes != nil {
			if *lYAML.Environments[buildValues.Branch].AutogenerateRoutes == false {
				autogenEnabled = false
			} else {
				autogenEnabled = true
			}
		}
		// check if autogenerated routes tls-acme disabled
		if lYAML.Routes.Autogenerate.TLSAcme != nil {
			if *lYAML.Routes.Autogenerate.TLSAcme == false {
				autogenTLSAcmeEnabled = false
			}
		}
		// check lagoon yaml for an override for this service
		if value, ok := lYAML.Environments[buildValues.Environment].Types[composeService]; ok {
			lagoonType = value
		}
		// check if the service has a specific override
		serviceAutogenerated := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.autogeneratedroute")
		if serviceAutogenerated != "" {
			if reflect.TypeOf(serviceAutogenerated).Kind() == reflect.String {
				vBool, err := strconv.ParseBool(serviceAutogenerated)
				if err == nil {
					autogenEnabled = vBool
				}
			}
		}
		// check if the service has a tls-acme specific override
		serviceAutogeneratedTLSAcme := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.autogeneratedroute.tls-acme")
		if serviceAutogeneratedTLSAcme != "" {
			if reflect.TypeOf(serviceAutogeneratedTLSAcme).Kind() == reflect.String {
				vBool, err := strconv.ParseBool(serviceAutogeneratedTLSAcme)
				if err == nil {
					autogenTLSAcmeEnabled = vBool
				}
			}
		}
		// check if the service has a deployment servicetype override
		serviceDeploymentServiceType := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.deployment.servicetype")
		if serviceDeploymentServiceType == "" {
			serviceDeploymentServiceType = composeService
		}

		// if there is a `lagoon.name` label on this service, this should be used as an override name
		lagoonOverrideName := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.name")
		if lagoonOverrideName != "" {
			// if there is an override name, check all other services already existing
			for _, service := range buildValues.Services {
				// if there is an existing service with this same override name, then disable autogenerated routes
				// for this service
				if service.OverrideName == lagoonOverrideName {
					autogenEnabled = false
				}
			}
		} else {
			// otherwise just set the override name to be the service name
			lagoonOverrideName = composeService
		}

		// check if the service has any persistent labels
		servicePersistentPath := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.persistent")
		if servicePersistentPath == "" {
			// if there is no persistent path, check if the service type has a default
			if val, ok := defaultServiceValues[lagoonType]; ok {
				servicePersistentPath = val["persistentPath"]
			}
		}
		servicePersistentName := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.persistent.name")
		if servicePersistentName == "" && servicePersistentPath != "" {
			// if there is a persistent path defined, then set the persistent name to be the compose service if no persistent name is provided
			servicePersistentName = composeService
		}
		servicePersistentSize := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.persistent.size")
		if servicePersistentSize == "" {
			// if there is no persistent size, check if the service type has a default
			if val, ok := defaultServiceValues[lagoonType]; ok {
				servicePersistentSize = val["persistentSize"]
			}
		}

		// if there are overrides defined in the lagoon API `LAGOON_SERVICE_TYPES`
		// handle those here
		if buildValues.ServiceTypeOverrides != nil {
			serviceTypesSplit := strings.Split(buildValues.ServiceTypeOverrides.Value, ",")
			for _, sType := range serviceTypesSplit {
				sTypeSplit := strings.Split(sType, ":")
				if sTypeSplit[0] == lagoonOverrideName {
					lagoonType = sTypeSplit[1]
				}
			}
		}

		// convert old service types to new service types from the old service map
		// this allows for adding additional values to the oldServiceMap that we can force to be anything else
		if val, ok := oldServiceMap[lagoonType]; ok {
			lagoonType = val
		}

		// if there are no overrides, and the type is none, then abort here, no need to proceed calculating the type
		if lagoonType == "none" {
			return ServiceValues{}, nil
		}

		// handle dbaas operator checks here
		dbaasEnvironment := buildValues.EnvironmentType
		if lagoonType == "mariadb" || lagoonType == "postgres" || lagoonType == "mongodb" {
			err := buildValues.DBaaSClient.CheckHealth(buildValues.DBaaSOperatorEndpoint)
			if err != nil {
				// @TODO eventually this error should be handled and fail a build, with a flag to override https://github.com/uselagoon/build-deploy-tool/issues/56
				// if !buildValues.DBaaSFallbackSingle {
				// 	return ServiceValues{}, fmt.Errorf("Unable to check the DBaaS endpoint %s: %v", buildValues.DBaaSOperatorEndpoint, err)
				// }
				if debug {
					fmt.Println(fmt.Sprintf("Unable to check the DBaaS endpoint %s, falling back to %s-single: %v", buildValues.DBaaSOperatorEndpoint, lagoonType, err))
				}
				// normally we would fall back to doing a cluster capability check, this is phased out in the build tool, it isn't reliable
				// and noone should be doing checks that way any more
				// the old bash check is the following
				// elif [[ "${CAPABILITIES[@]}" =~ "mariadb.amazee.io/v1/MariaDBConsumer" ]] && ! checkDBaaSHealth ; then
				lagoonType = fmt.Sprintf("%s-single", lagoonType)
			} else {
				// if there is a `lagoon.%s-dbaas.environment` label on this service, this should be used as an the environment type for the dbaas
				dbaasLabelOverride := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, fmt.Sprintf("lagoon.%s-dbaas.environment", lagoonType))
				if dbaasLabelOverride != "" {
					dbaasEnvironment = dbaasLabelOverride
				}

				// @TODO: maybe phase this out?
				// if value, ok := lYAML.Environments[buildValues.Environment].Overrides[composeService][mariadb][mariadb-dbaas].Environment; ok {
				// this isn't documented in the lagoon.yml, and it looks like a failover from days past.
				// 	lagoonType = value
				// }

				// if there are overrides defined in the lagoon API `LAGOON_DBAAS_ENVIRONMENT_TYPES`
				// handle those here
				exists, err := getDBaasEnvironment(buildValues, &dbaasEnvironment, lagoonOverrideName, lagoonType, debug)
				if err != nil {
					// @TODO eventually this error should be handled and fail a build, with a flag to override https://github.com/uselagoon/build-deploy-tool/issues/56
					// if !buildValues.DBaaSFallbackSingle {
					// 	return ServiceValues{}, err
					// }
					if debug {
						fmt.Println(fmt.Sprintf(
							"There was an error checking DBaaS endpoint %s, falling back to %s-single: %v",
							buildValues.DBaaSOperatorEndpoint, lagoonType, err,
						))
					}
				}

				// if the requested dbaas environment exists, then set the type to be the requested type with `-dbaas`
				if exists {
					lagoonType = fmt.Sprintf("%s-dbaas", lagoonType)
				} else {
					// otherwise fallback to -single (if DBaaSFallbackSingle is enabled, otherwise it will error out prior)
					lagoonType = fmt.Sprintf("%s-single", lagoonType)
				}
			}
		}

		// check if this service is one that supports autogenerated routes
		if !helpers.Contains(supportedAutogeneratedTypes, lagoonType) {
			autogenEnabled = false
			autogenTLSAcmeEnabled = false
		}

		// create the service values
		cService := ServiceValues{
			Name:                       composeService,
			OverrideName:               lagoonOverrideName,
			Type:                       lagoonType,
			AutogeneratedRoutesEnabled: autogenEnabled,
			AutogeneratedRoutesTLSAcme: autogenTLSAcmeEnabled,
			DBaaSEnvironment:           dbaasEnvironment,
			PersistentVolumePath:       servicePersistentPath,
			PersistentVolumeName:       servicePersistentName,
			PersistentVolumeSize:       servicePersistentSize,
		}
		// check if the service has a service port override (this only applies to basic(-persistent))
		servicePortOverride := lagoon.CheckServiceLagoonLabel(composeServiceValues.Labels, "lagoon.service.port")
		if servicePortOverride != "" {
			sPort, err := strconv.Atoi(servicePortOverride)
			if err != nil {
				return ServiceValues{}, fmt.Errorf(
					"The provided service port %s for service %s is not a valid integer: %v",
					servicePortOverride, composeService, err,
				)
			}
			cService.ServicePort = int32(sPort)
		}
		return cService, nil
	}
}

// getDBaasEnvironment will check the dbaas provider to see if an environment exists or not
func getDBaasEnvironment(
	buildValues *BuildValues,
	dbaasEnvironment *string,
	lagoonOverrideName,
	lagoonType string,
	debug bool,
) (bool, error) {
	if buildValues.DBaaSEnvironmentTypeOverrides != nil {
		dbaasEnvironmentTypeSplit := strings.Split(buildValues.DBaaSEnvironmentTypeOverrides.Value, ",")
		for _, sType := range dbaasEnvironmentTypeSplit {
			sTypeSplit := strings.Split(sType, ":")
			if sTypeSplit[0] == lagoonOverrideName {
				*dbaasEnvironment = sTypeSplit[1]
			}
		}
	}
	exists, err := buildValues.DBaaSClient.CheckProvider(buildValues.DBaaSOperatorEndpoint, lagoonType, *dbaasEnvironment)
	if err != nil {
		return exists, fmt.Errorf("There was an error checking DBaaS endpoint %s: %v", buildValues.DBaaSOperatorEndpoint, err)
	}
	return exists, nil
}

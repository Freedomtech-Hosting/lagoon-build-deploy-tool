package lagoon

import (
	"encoding/json"
	"reflect"
	"strconv"

	"github.com/uselagoon/lagoon-routegen/internal/helpers"
)

// RoutesV2 is the new routes definition
type RoutesV2 struct {
	Routes []RouteV2 `json:"routes"`
}

// RouteV2 is the new route definition
type RouteV2 struct {
	Domain         string            `json:"domain"`
	Service        string            `json:"service"`
	TLSAcme        *bool             `json:"tls-acme"`
	Migrate        *bool             `json:"migrate,omitempty"`
	Insecure       *string           `json:"insecure,omitempty"`
	HSTS           *string           `json:"hsts,omitempty"`
	MonitoringPath string            `json:"monitoring-path,omitempty"`
	Fastly         Fastly            `json:"fastly,omitempty"`
	Annotations    map[string]string `json:"annotations"`
}

// Ingress represents a Lagoon route.
type Ingress struct {
	TLSAcme        *bool             `json:"tls-acme,omitempty"`
	Migrate        *bool             `json:"migrate,omitempty"`
	Insecure       *string           `json:"insecure,omitempty"`
	HSTS           *string           `json:"hsts,omitempty"`
	MonitoringPath string            `json:"monitoring-path,omitempty"`
	Fastly         Fastly            `json:"fastly,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
}

// Route can be either a string or a map[string]Ingress, so we must
// implement a custom unmarshaller.
type Route struct {
	Name      string
	Ingresses map[string]Ingress
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *Route) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.Name); err == nil {
		return nil
	}
	if err := json.Unmarshal(data, &r.Ingresses); err != nil {
		// some things in .lagoon.yml can be defined as a bool or string and lagoon builds don't care
		// but types are more strict, so this unmarshaler attempts to change between the two types
		// that can be bool or string
		tmpMap := map[string]interface{}{}
		json.Unmarshal(data, &tmpMap)
		for k := range tmpMap {
			if _, ok := tmpMap[k].(map[string]interface{})["tls-acme"]; ok {
				if reflect.TypeOf(tmpMap[k].(map[string]interface{})["tls-acme"]).Kind() == reflect.String {
					vBool, err := strconv.ParseBool(tmpMap[k].(map[string]interface{})["tls-acme"].(string))
					if err == nil {
						tmpMap[k].(map[string]interface{})["tls-acme"] = vBool
					}
				}
			}
			if _, ok := tmpMap[k].(map[string]interface{})["fastly"]; ok {
				if reflect.TypeOf(tmpMap[k].(map[string]interface{})["fastly"].(map[string]interface{})["watch"]).Kind() == reflect.String {
					vBool, err := strconv.ParseBool(tmpMap[k].(map[string]interface{})["fastly"].(map[string]interface{})["watch"].(string))
					if err == nil {
						tmpMap[k].(map[string]interface{})["fastly"].(map[string]interface{})["watch"] = vBool
					}
				}
			}
		}
		newData, _ := json.Marshal(tmpMap)
		return json.Unmarshal(newData, &r.Ingresses)
	}
	return json.Unmarshal(data, &r.Ingresses)
}

// GenerateRoutesV2 generate routesv2 definitions from lagoon route mappings
func GenerateRoutesV2(genRoutes *RoutesV2, routeMap map[string][]Route, variables []EnvironmentVariable, activeStandby bool) {
	for rName, lagoonRoutes := range routeMap {
		for _, lagoonRoute := range lagoonRoutes {
			newRoute := &RouteV2{}
			// set the defaults for routes
			newRoute.TLSAcme = helpers.BoolPtr(true)
			newRoute.Insecure = helpers.StrPtr("Redirect")
			newRoute.MonitoringPath = "/"
			newRoute.HSTS = helpers.StrPtr("null")
			newRoute.Annotations = map[string]string{}
			newRoute.Fastly.ServiceID = ""
			newRoute.Fastly.Watch = false
			if activeStandby {
				newRoute.Migrate = helpers.BoolPtr(true)
			}
			if lagoonRoute.Name == "" {
				// this route from the lagoon route map contains field overrides
				// update them from the defaults in this case
				for iName, ingress := range lagoonRoute.Ingresses {
					newRoute.Domain = iName
					newRoute.Service = rName
					newRoute.Fastly = ingress.Fastly
					if ingress.Annotations != nil {
						newRoute.Annotations = ingress.Annotations
					}
					if ingress.TLSAcme != nil {
						newRoute.TLSAcme = ingress.TLSAcme
					}
					if ingress.Insecure != nil {
						newRoute.Insecure = ingress.Insecure
					}
					if ingress.HSTS != nil {
						newRoute.HSTS = ingress.HSTS
					}
				}
			} else {
				// this route is just a domain
				// keep the defaults, just set the name and service
				newRoute.Domain = lagoonRoute.Name
				newRoute.Service = rName
			}
			// generate the fastly configuration for this route
			fConfig, err := GenerateFastlyConfiguration("", newRoute.Fastly.ServiceID, newRoute.Domain, variables)
			if err != nil {
			}
			newRoute.Fastly = fConfig

			genRoutes.Routes = append(genRoutes.Routes, *newRoute)
		}
	}
}

// MergeRoutesV2 merge routes from the API onto the previously generated routes.
func MergeRoutesV2(genRoutes RoutesV2, apiRoutes RoutesV2) RoutesV2 {
	finalRoutes := &RoutesV2{}
	existsInAPI := false
	// replace any routes from the lagoon yaml with ones from the api
	// this only modifies ones that exist in lagoon yaml
	for _, route := range genRoutes.Routes {
		add := RouteV2{}
		for _, aRoute := range apiRoutes.Routes {
			if aRoute.Domain == route.Domain {
				existsInAPI = true
				add = aRoute
				add.Fastly = aRoute.Fastly
				if aRoute.TLSAcme != nil {
					add.TLSAcme = aRoute.TLSAcme
				} else {
					add.TLSAcme = helpers.BoolPtr(true)
				}
				if aRoute.Insecure != nil {
					add.Insecure = aRoute.Insecure
				} else {
					add.Insecure = helpers.StrPtr("Redirect")
				}
				if aRoute.HSTS != nil {
					add.HSTS = aRoute.HSTS
				} else {
					add.HSTS = helpers.StrPtr("null")
				}
				if aRoute.Annotations != nil {
					add.Annotations = aRoute.Annotations
				} else {
					add.Annotations = map[string]string{}
				}
			}
		}
		if existsInAPI {
			finalRoutes.Routes = append(finalRoutes.Routes, add)
			existsInAPI = false
		} else {
			finalRoutes.Routes = append(finalRoutes.Routes, route)
		}
	}
	// add any that exist in the api only to the final routes list
	for _, aRoute := range apiRoutes.Routes {
		add := RouteV2{}
		for _, route := range finalRoutes.Routes {
			add = aRoute
			add.Fastly = aRoute.Fastly
			if aRoute.TLSAcme != nil {
				add.TLSAcme = aRoute.TLSAcme
			} else {
				add.TLSAcme = helpers.BoolPtr(true)
			}
			if aRoute.Insecure != nil {
				add.Insecure = aRoute.Insecure
			} else {
				add.Insecure = helpers.StrPtr("Redirect")
			}
			if aRoute.HSTS != nil {
				add.HSTS = aRoute.HSTS
			} else {
				add.HSTS = helpers.StrPtr("null")
			}
			if aRoute.Annotations != nil {
				add.Annotations = aRoute.Annotations
			} else {
				add.Annotations = map[string]string{}
			}
			if aRoute.Domain == route.Domain {
				existsInAPI = true
			}
		}
		if existsInAPI {
			existsInAPI = false
		} else {
			finalRoutes.Routes = append(finalRoutes.Routes, add)
		}
	}
	return *finalRoutes
}
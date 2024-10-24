package utils

import (
	"log"
	"strings"
)

func ValidateResponse(respMap map[string]string, expectedArgs ...string) bool {
	cond1 := respMap != nil && respMap["network_name"] == "baadal" && len(respMap["client_version"]) > 0
	if !cond1 {
		return false
	}

	if len(expectedArgs) > 0 {
		for _, val := range expectedArgs {
			item := respMap[val]
			cond := len(item) > 0
			if !cond {
				log.Printf("Expected arg (%s) not found in resp: %v\n", val, respMap)
				return false
			}
		}
	}

	return true
}

/*
Creates a map object from a standard protocol response.

E.g. "network_name: baadal; client_version: 0.1.0" gets converted into

map[client_version:0.1.0 network_name:baadal]
*/
func ResponseMap(resp string) map[string]string {
	if resp == "" {
		return nil
	}

	parts := strings.Split(resp, ";")
	if len(parts) == 0 {
		return nil
	}

	respMap := make(map[string]string)
	for _, p := range parts {
		parts2 := strings.Split(strings.TrimSpace(p), ":")
		if len(parts2) != 2 {
			return nil
		}
		k := strings.TrimSpace(parts2[0])
		v := strings.TrimSpace(parts2[1])
		respMap[k] = v
	}

	if len(respMap) == 0 {
		return nil
	}

	return respMap
}

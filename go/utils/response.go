package utils

import "strings"

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

	m := make(map[string]string)
	for _, p := range parts {
		parts2 := strings.Split(strings.TrimSpace(p), ":")
		if len(parts2) != 2 {
			return nil
		}
		k := strings.TrimSpace(parts2[0])
		v := strings.TrimSpace(parts2[1])
		m[k] = v
	}

	if len(m) == 0 {
		return nil
	}

	return m
}

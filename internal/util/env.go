package util

import (
	"os"
	"strconv"
)

func GetAuthKey() string {
	return os.Getenv(AuthKeyEnvVar)
}

func GetProxyHttpPort() int {
	port, err := strconv.Atoi(os.Getenv(ProxyHttpPortEnvVar))
	if err != nil {
		return DefaultPort
	}
	return port
}

package util

const ProgramName = "tsgrok"

var ProgramVersion = "dev" // Will be overwritten by goreleaser
var Commit = "none"        // Will be overwritten by goreleaser
var Date = "unknown"       // Will be overwritten by goreleaser

const DefaultPort = 4141

const AuthKeyEnvVar = "TSGROK_AUTHKEY"               // env var for auth key
const ProxyHttpPortEnvVar = "TSGROK_PROXY_HTTP_PORT" // env var for proxy http port, defaults to DefaultPort

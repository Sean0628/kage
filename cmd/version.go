package cmd

import "runtime/debug"

var version = "dev"

func versionString() string {
	if version != "" && version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		return version
	}

	return info.Main.Version
}

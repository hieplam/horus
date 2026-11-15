// Package version holds the one version of the horus program.
//
// A release build sets Version with the linker flag
//
//	-ldflags "-X github.com/hieplam/horus/internal/version.Version=1.2.3"
//
// A build without the flag has the version "dev".
package version

// Version is the version of this build.
var Version = "dev"

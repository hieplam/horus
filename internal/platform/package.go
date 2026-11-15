// Package: a system package that the ComfyUI install of an OS needs, and the
// command of the package manager that installs it.
package platform

// Package is one system package: what it gives, where the package manager
// puts it, and the command that installs it. It is plain data. The backend of
// each OS gives its own (internal/backend, SetupNeeds).
type Package struct {
	// Name is the name of the package for a person, for example "7-Zip".
	Name string
	// Program is the program that the package gives, without ".exe", for
	// example "7z" or "python3.12".
	Program string
	// Dirs are the folders where the package manager puts Program, in
	// order. A caller looks there after the install, because the PATH of a
	// running process does not change.
	Dirs []string
	// InManagerDir is true when the package manager also puts Program in its
	// own folder (the bin folder of Homebrew).
	InManagerDir bool
	// Manager is the program of the package manager, for example "winget".
	Manager string
	// Args are the arguments of Manager that install the package.
	Args []string
	// URL is where a person gets the package by hand.
	URL string
}

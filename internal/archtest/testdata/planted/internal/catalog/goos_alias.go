// Planted: it uses runtime.GOOS through another import name.
package catalog

import rt "runtime"

func isWindows() bool { return rt.GOOS == "windows" }

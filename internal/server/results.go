// Result models that more than one tool uses. Part of the port of
// horus/server.py.
package server

// InstallFile is one model file of a workflow and its state. The keys are in
// the order of InstallFile of server.py.
type InstallFile struct {
	Role   string  `json:"role"`
	File   string  `json:"file"`
	Bytes  int64   `json:"bytes"`
	State  string  `json:"state"`
	Path   *string `json:"path"`
	Detail *string `json:"detail"`
}

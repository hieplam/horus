// Result models that more than one tool uses. Part of the port of
// horus/server.py.
package mcpserver

import "github.com/hieplam/horus/internal/models"

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

// fileRows is _file_rows of server.py: the items of one kind whose status is
// in states, as InstallFile rows.
func fileRows(items []models.Item, kind string, states map[string]string) []InstallFile {
	rows := []InstallFile{}
	for _, i := range items {
		state, ok := states[i.Status]
		if i.Kind != kind || !ok {
			continue
		}
		row := InstallFile{Role: i.Role, File: i.File, Bytes: max(0, i.Bytes), State: state}
		if i.Path != "" {
			row.Path = ptr(i.Path)
		}
		if i.Detail != "" {
			row.Detail = ptr(i.Detail)
		}
		rows = append(rows, row)
	}
	return rows
}

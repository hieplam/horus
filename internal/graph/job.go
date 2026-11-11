package graph

// Job is what ComfyUI reports about a graph that it runs: the answer of one
// poll of its history. The adapter internal/comfy reads it from the wire
// format. The service internal/generate decides what to do with it.
type Job struct {
	// Done is true when the job finished with success.
	Done bool
	// Images are the output images of a finished job, in the order of the
	// output nodes. Empty while the job runs.
	Images []Image
	// BackendSeconds is the time from the start to the end of the job on
	// the server, rounded to 0.1 s. nil when the server did not report it.
	BackendSeconds *float64
}

// Image is one output image of a job, as the server names it.
type Image struct {
	Filename  string
	Subfolder string
	Type      string
}

// Queue is where ComfyUI keeps one job in its queue. The adapter
// internal/comfy reads it from the wire format. The service
// internal/generate and Cancel read the fields.
type Queue struct {
	// Running is true while the server runs the job.
	Running bool
	// Position is the place of a waiting job in the queue, counted from 1.
	// It is 0 when the job is not waiting.
	Position int
}

// Pending reports whether the job waits in the queue.
func (q Queue) Pending() bool { return q.Position > 0 }

// Absent reports whether the server does not list the job at all.
func (q Queue) Absent() bool { return !q.Running && q.Position == 0 }

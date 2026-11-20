// The wiring: cmd/horus creates the adapters and gives them to the services.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hieplam/horus/internal/backend"
	"github.com/hieplam/horus/internal/cli"
	"github.com/hieplam/horus/internal/comfy"
	"github.com/hieplam/horus/internal/devtools"
	"github.com/hieplam/horus/internal/doctor"
	"github.com/hieplam/horus/internal/download"
	"github.com/hieplam/horus/internal/generate"
	"github.com/hieplam/horus/internal/jobdata"
	"github.com/hieplam/horus/internal/jobs"
	"github.com/hieplam/horus/internal/mcpserver"
	"github.com/hieplam/horus/internal/modelfiles"
	"github.com/hieplam/horus/internal/platform"
	"github.com/hieplam/horus/internal/postgres"
	"github.com/hieplam/horus/internal/provision"
	"github.com/hieplam/horus/internal/settings"
	"github.com/hieplam/horus/internal/speak"
	"github.com/hieplam/horus/internal/speechsetup"
	"github.com/hieplam/horus/internal/speechworker"
	"github.com/hieplam/horus/internal/version"
	"github.com/hieplam/horus/internal/webui"
	"github.com/hieplam/horus/internal/worker"
)

// newImages makes the image service on the ComfyUI adapter and the backend.
// An empty url means the HORUS_COMFYUI_URL setting, read at each request.
// The backend starts ComfyUI at the same address, in the HORUS_COMFYUI_DIR
// folder.
func newImages(url string) *generate.Service { return imagesFor(url, seams{}) }

// imagesFor is newImages with the test seams s: s.goos picks the backend of
// another OS.
func imagesFor(url string, s seams) *generate.Service {
	return generate.New(comfy.New(url), backendFor(s.goos, backend.Place{URL: url}, backend.Archive{}))
}

// backendFor gives the backend of the OS goos, or of this OS when goos is "".
func backendFor(goos string, p backend.Place, a backend.Archive) backend.Backend {
	if goos != "" {
		return backend.NewFor(goos, p, a)
	}
	return backend.New(p, a)
}

// imagesAt is the image service for the -url flag of run-illustration.
func imagesAt(url string) cli.Images { return newImages(url) }

// newBackend makes the backend of this OS for the flags of a command. An
// empty field means the setting, read at each call.
func newBackend(p cli.BackendPlace) backend.Backend {
	return backend.New(backend.Place{Dir: p.Dir, URL: p.ServerURL, Timeout: p.Timeout},
		backend.Archive{ArchivePath: p.ArchivePath, SevenZip: p.SevenZip, KeepArchive: p.KeepArchive})
}

// backendAt is the backend for the flags of install-comfyui, start-comfyui
// and stop-comfyui.
func backendAt(p cli.BackendPlace) cli.Backend { return newBackend(p) }

// takesArchive tells install-comfyui if the backend of this OS takes the
// flags of the archive (the Windows portable build).
var takesArchive = backend.TakesArchive()

// installAt is the ComfyUI install for the flag -comfyui-dir of
// sync-workflows.
func installAt(dir string) devtools.Install { return newBackend(cli.BackendPlace{Dir: dir}) }

// seams are the test seams of the setup service and the image service. The zero value is the real
// behaviour.
type seams struct {
	download download.Options  // the base options of each download
	now      func() time.Time  // the time in the name of a backup of extra_model_paths.yaml
	goos     string            // the OS whose backend installs and starts ComfyUI; "" means this OS
	install  provision.Backend // the ComfyUI install of horus setup; nil means the backend of goos
	// installer is the PATH entry and the program file of horus uninstall;
	// nil means the functions of internal/platform.
	installer provision.Installer
}

// newSetup makes the setup service on the model files, the downloader, the
// ComfyUI and the install of place. An empty field of place means the
// setting, read at each call.
func newSetup(place cli.SetupPlace, s seams) *provision.Service {
	files := &modelfiles.Store{ComfyUIDir: place.ComfyUIDir, ModelsDir: place.ModelsDir, TextEncoderDir: place.TextEncoderDir, Now: s.now}
	var install provision.Backend = backendFor(s.goos, backend.Place{Dir: place.ComfyUIDir, URL: place.ServerURL}, backend.Archive{})
	if s.install != nil {
		install = s.install
	}
	needs := backend.SetupNeeds(s.goos)
	return provision.New(files, downloader{s.download}, comfy.New(place.ServerURL), install, place.ServerURL).
		WithSystem(provision.System{Programs: platform.Programs{}, Package: needs.Package, ComfyCLI: needs.ComfyCLI,
			Speech: platform.SpeechPackagesFor(s.goos)})
}

// setupAt is the setup service for the flags of horus workflow.
func setupAt(place cli.SetupPlace) cli.Setup { return newSetup(place, seams{}) }

// setupStepsAt is the setup service for the flags of horus setup.
func setupStepsAt(place cli.SetupPlace) cli.SetupSteps { return newSetup(place, seams{}) }

// newUninstall makes the setup service of place with the parts of horus
// uninstall: the backend of goos (its folder, its server and its stop), the
// OS (the PATH entry and the program file) and the programs claude and
// codex.
func newUninstall(place cli.SetupPlace, s seams) *provision.Service {
	var osPart provision.Installer = installerOS{}
	if s.installer != nil {
		osPart = s.installer
	}
	comfyInstall := backendFor(s.goos, backend.Place{Dir: place.ComfyUIDir, URL: place.ServerURL}, backend.Archive{})
	return newSetup(place, s).WithUninstall(provision.UninstallSystem{Comfy: comfyInstall, OS: osPart, Programs: platform.Programs{}})
}

// uninstallAt is the service for the flags of horus uninstall.
func uninstallAt(place cli.SetupPlace) cli.Uninstaller { return newUninstall(place, seams{}) }

// installerOS gives the uninstall the functions of internal/platform for
// the PATH entry of the install scripts and the file of this program.
type installerOS struct{}

func (installerOS) Program() (string, error) { return os.Executable() }

func (installerOS) PathEntry(program string) ([]string, error) {
	home, _ := os.UserHomeDir()
	return platform.PathEntry(program, home)
}

func (installerOS) RemovePathEntry(program string) error {
	home, _ := os.UserHomeDir()
	return platform.RemovePathEntry(program, home)
}

func (installerOS) RemoveProgram() error {
	_, err := platform.RemoveSelf()
	return err
}

// downloader gives internal/provision the downloader of internal/download.
// The service cannot name the types of an adapter, so this glue copies the
// request in, and the kind and the reason lines of a failure out.
type downloader struct{ base download.Options }

// kinds maps the kind of a failed download to the kind of the service.
var kinds = map[download.Kind]provision.DownloadKind{
	download.KindFailed:       provision.DownloadFailed,
	download.KindUnauthorized: provision.DownloadUnauthorized,
	download.KindForbidden:    provision.DownloadForbidden,
	download.KindDiskFull:     provision.DownloadDiskFull,
	download.KindConflict:     provision.DownloadConflict,
	download.KindBadHash:      provision.DownloadBadHash,
}

func (d downloader) Download(ctx context.Context, f provision.Fetch, out io.Writer) (bool, error) {
	o := d.base
	o.URL, o.Dest, o.ExpectedBytes, o.SHA256, o.Out = f.URL, f.Dest, f.Bytes, f.SHA256, out
	res, err := download.Download(ctx, o)
	var de *download.Error
	if errors.As(err, &de) {
		return false, &provision.DownloadError{Kind: kinds[de.Kind], Msg: de.Msg, Reasons: de.Reasons}
	}
	return res.Skipped, err
}

// newSpeech makes the speak service on the speech worker adapter and the
// programs of this machine. Each speech worker that it starts ends when life
// ends. The error output of a worker goes to the error output of this
// process, so the log of the MCP client has it.
func newSpeech(life context.Context) *speak.Service {
	return speak.New(func(p speechsetup.Place) speak.Worker {
		return speechGlue{speechworker.New(life, speechworker.Config{Python: p.Python, Script: p.Script, Models: p.Models, Log: os.Stderr})}
	}, platform.Programs{})
}

// speechGlue gives internal/speak the adapter internal/speechworker. The
// service cannot name the types of an adapter, so this glue copies the request
// in, and the answer and the kind of a failure out.
type speechGlue struct{ w *speechworker.Worker }

func (g speechGlue) Speak(ctx context.Context, t speak.Text) (speak.Speech, error) {
	sp, err := g.w.Speak(ctx, speechworker.Text{Text: t.Text, Voice: t.Voice, Out: t.Out, Format: t.Format})
	var we *speechworker.Error
	switch {
	case errors.As(err, &we):
		return speak.Speech{}, &speak.WorkerError{Stopped: errors.Is(err, speechworker.ErrStopped), Message: we.Message, Output: we.Stderr}
	case errors.Is(err, speechworker.ErrClosed):
		return speak.Speech{}, &speak.WorkerError{Stopped: true, Message: err.Error()}
	case err != nil:
		return speak.Speech{}, err
	}
	return speak.Speech{Path: sp.Path, Bytes: sp.Bytes, Format: sp.Format, AudioSeconds: sp.AudioSeconds, Transcript: sp.Transcript, Started: sp.Started}, nil
}

func (g speechGlue) Running() bool { return g.w.PID() != 0 }

func (g speechGlue) Close() error { return g.w.Close() }

// doctorAt is the doctor service, and the setup service for the model
// check, for the flags of horus doctor. The model check reads the address of
// ComfyUI from the setting, as horus workflow check does without -server-url.
func doctorAt(p cli.DoctorPlace) (cli.Doctor, cli.Setup) {
	install := newBackend(cli.BackendPlace{Dir: p.ComfyUIDir, ServerURL: p.ServerURL})
	setup := setupAt(cli.SetupPlace{ComfyUIDir: p.ComfyUIDir, ModelsDir: p.ModelsDir, TextEncoderDir: p.TextEncoderDir})
	return doctor.New(comfy.New(p.ServerURL), install, platform.Programs{}), setup
}

// runWorker is horus worker. It connects the job store and the broker of
// HORUS_DATABASE_URL, applies the schema horus, and runs the loop of
// internal/worker with the ComfyUI of HORUS_COMFYUI_URL until ctx ends. Each
// start gets a new worker id. The worker never starts ComfyUI.
func runWorker(ctx context.Context, out io.Writer) error {
	url := settings.DatabaseURL()
	if url == "" {
		return errors.New("HORUS_DATABASE_URL is not set: the worker needs the job store and the broker in Postgres (docs/mcp.md#the-worker)")
	}
	slots, err := settings.WorkerConcurrency()
	if err != nil {
		return err
	}
	days, err := settings.JobRetentionDays()
	if err != nil {
		return err
	}
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	queue := jobs.New(postgres.NewStore(db), postgres.NewBroker(db, postgres.BrokerOptions{}), jobs.Config{Slots: slots})
	id, err := jobs.NewID(time.Now(), rand.Reader)
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	cfg := worker.Config{ID: id, Host: host, PID: os.Getpid(), Version: version.Version, Slots: slots,
		Retention: time.Duration(days) * 24 * time.Hour, Log: out}
	return worker.Run(ctx, cfg, queue, generate.New(comfy.New(""), noStart{}))
}

// noStart is the backend of the worker. The worker runs each job with
// generate.Options.NoStart, so a call of Start is a bug.
type noStart struct{}

func (noStart) Start(context.Context, io.Writer) error {
	return errors.New("horus worker never starts ComfyUI: the owner starts it")
}

// stackOf gives the parts of the readiness of horus in the stack, and the
// function that closes its database pool. The checks of horus-db and
// horus-worker use the database of HORUS_DATABASE_URL, and the pool opens no
// connection before the first check, so a process that starts while horus-db
// is down reports it. The check of ComfyUI asks HORUS_COMFYUI_URL. apiURL is
// the address of horus api to check, or "" in the process of horus api.
func stackOf(ctx context.Context, apiURL string) (mcpserver.Stack, func()) {
	stack, _, done := stackWith(ctx, apiURL, jobs.Config{})
	return stack, done
}

// stackWith is stackOf, and it also gives the job queue service with the
// settings cfg. The service is nil when the database URL cannot be used:
// then the checks of horus-db and horus-worker fail with the cause.
func stackWith(ctx context.Context, apiURL string, cfg jobs.Config) (mcpserver.Stack, *jobs.Service, func()) {
	stack := mcpserver.Stack{Images: stackImages(), APIURL: apiURL}
	url := settings.DatabaseURL()
	if url == "" {
		stack.Queue = downQueue("HORUS_DATABASE_URL is not set: horus ready runs in the stack, for example docker exec horus-api horus ready")
		return stack, nil, func() {}
	}
	db, err := postgres.Open(ctx, url)
	if err != nil {
		stack.Queue = downQueue(err.Error())
		return stack, nil, func() {}
	}
	svc := jobs.New(postgres.NewStore(db), postgres.NewBroker(db, postgres.BrokerOptions{}), cfg).WithHealth(db)
	stack.Queue = svc
	return stack, svc, db.Close
}

// stackImages is the image service of a process of the stack: it never
// starts ComfyUI. horus mcp in the stack uses only its argument check and
// its check of ComfyUI: the worker runs each job.
func stackImages() *generate.Service { return generate.New(comfy.New(""), noStart{}) }

// stackSetup is the setup service of an API process of the stack (ADR 0041).
// Its model files are only in HORUS_MODELS_DIR, the folder of horus-api where
// horus stack-mounts mounts the host folder that ComfyUI reads. ComfyUI runs
// on the host (hostComfyUI), so a plan has no ComfyUI step and no system
// package step: only model files. The check of the model files asks the
// ComfyUI of HORUS_COMFYUI_URL.
func stackSetup() *provision.Service { return stackSetupFor(seams{}) }

// stackSetupFor is stackSetup with the test seams s: the downloads of
// s.download.
func stackSetupFor(s seams) *provision.Service {
	files := &modelfiles.Store{OnlyModelsDir: true}
	return provision.New(files, downloader{s.download}, comfy.New(""), hostComfyUI{}, "")
}

// hostComfyUI is the ComfyUI install of the setup service of the stack. The
// owner installs and starts ComfyUI on the host (ADR 0029), so for the plan
// ComfyUI is installed, and the stack never installs it.
type hostComfyUI struct{}

func (hostComfyUI) Installed() (string, bool) {
	return "the ComfyUI of the host at " + settings.ComfyURL(), true
}

func (hostComfyUI) Install(context.Context, io.Writer) error {
	return errors.New("horus in the stack never installs ComfyUI: the owner installs it on the host")
}

// stackModels is the models folder of an API process of the stack:
// HORUS_MODELS_DIR, its host folder HORUS_MODELS_HOST_DIR and the system of
// the host HORUS_HOST_OS, as horus stack-mounts writes them.
func stackModels() *mcpserver.Models {
	return mcpserver.ModelsOf(settings.ModelsDir(), settings.ModelsHostDir(), settings.HostOS())
}

// queueConfig gives the settings of the job queue of an API process:
// HORUS_QUEUE_MAX, HORUS_QUEUE_TIMEOUT, HORUS_WORKER_CONCURRENCY (for the
// estimated start) and HORUS_RUN_ESTIMATE.
func queueConfig() (jobs.Config, error) {
	max, err := settings.QueueMax()
	if err != nil {
		return jobs.Config{}, err
	}
	timeout, err := settings.QueueTimeout()
	if err != nil {
		return jobs.Config{}, err
	}
	slots, err := settings.WorkerConcurrency()
	if err != nil {
		return jobs.Config{}, err
	}
	run, err := settings.RunEstimate()
	if err != nil {
		return jobs.Config{}, err
	}
	return jobs.Config{QueueMax: max, QueueTimeout: time.Duration(timeout) * time.Second, Slots: slots,
		DefaultRun: time.Duration(run) * time.Second}, nil
}

// stackMCP gives the parts of horus mcp in the stack: the readiness gate and
// the job queue of generate_image with the shared folders of
// HORUS_SHARED_DIRS. done closes the database pool.
func stackMCP(ctx context.Context) (mcpserver.Options, func(), error) {
	opts, _, done, err := stackParts(ctx, localAPI())
	return opts, done, err
}

// stackParts gives the parts of an API process of the stack: the readiness
// gate and the job queue of the MCP tools (opts), and the readiness of the 4
// dependencies with its job queue service (stack). apiURL is the address of
// horus api to check, or "" in the process of horus api. done closes the
// database pool.
func stackParts(ctx context.Context, apiURL string) (mcpserver.Options, mcpserver.Stack, func(), error) {
	cfg, err := queueConfig()
	if err != nil {
		return mcpserver.Options{}, mcpserver.Stack{}, nil, err
	}
	dirs, err := settings.SharedDirs()
	if err != nil {
		return mcpserver.Options{}, mcpserver.Stack{}, nil, err
	}
	stack, svc, done := stackWith(ctx, apiURL, cfg)
	if svc == nil {
		done()
		return mcpserver.Options{}, mcpserver.Stack{}, nil, fmt.Errorf("the job queue has no database: %s", stack.Ready(ctx).Text())
	}
	q := &mcpserver.Queue{Jobs: svc, Paths: jobs.PathsOf(dirs, settings.HostOS()), QueueTimeout: cfg.QueueTimeout}
	return mcpserver.Options{Ready: stack.Ready, Queue: q, Models: stackModels()}, stack, done, nil
}

// downQueue is the job queue when the database cannot be used: the checks of
// horus-db and horus-worker fail with the cause.
type downQueue string

func (q downQueue) Checks(context.Context) []jobdata.CheckResult {
	return []jobdata.CheckResult{{Dependency: jobdata.DependencyDB, Detail: string(q)}, {Dependency: jobdata.DependencyWorker, Detail: string(q)}}
}

// Summary, Depth and Listen are the reads of the Horus UI. They fail with the
// cause.
func (q downQueue) Summary(context.Context, time.Time) (jobdata.Summary, error) {
	return jobdata.Summary{}, errors.New(string(q))
}

func (q downQueue) Depth(context.Context, string) (int, error) { return 0, errors.New(string(q)) }

func (q downQueue) Listen(context.Context) (<-chan jobdata.Event, error) {
	return nil, errors.New(string(q))
}

// localAPI is the address of horus api in this container (or on this
// machine): http://127.0.0.1:<HORUS_API_PORT>.
func localAPI() string {
	port, err := settings.APIPort()
	if err != nil {
		port = 8190
	}
	return "http://127.0.0.1:" + strconv.Itoa(port)
}

// readyEvery is how often horus api runs the 4 checks to log a change of
// the readiness.
const readyEvery = 5 * time.Second

// runAPI is horus api: the HTTP server of horus-api on listen until ctx ends:
// /mcp (internal/mcpserver: the MCP tools over Streamable HTTP, with the
// bearer token of HORUS_API_TOKEN), and the routes of internal/webui (the
// Horus UI, /readyz and /livez). It writes one log line when it starts, when
// the readiness changes, and when it stops. The log never has the token.
func runAPI(ctx context.Context, listen string, out io.Writer) error {
	if settings.DatabaseURL() == "" {
		return errors.New("HORUS_DATABASE_URL is not set: horus api serves horus in the stack (docs/setup.md#run-horus-as-a-stack)")
	}
	port, err := settings.APIPort()
	if err != nil {
		return err
	}
	slots, err := settings.WorkerConcurrency()
	if err != nil {
		return err
	}
	limit, err := settings.GenerateTimeout()
	if err != nil {
		return err
	}
	opts, stack, done, err := stackParts(ctx, "")
	if err != nil {
		return err
	}
	defer done()
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	ports := webui.Ports(port, ln.Addr().String())
	token := settings.APIToken()
	setup := stackSetup()
	queue, _ := stack.Queue.(webui.Queue)
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.NewHTTP(stackImages(), setup, opts, mcpserver.HTTPConfig{Token: token, Ports: ports}))
	mux.Handle("/", webui.Handler(webui.Config{
		Ready:        func(ctx context.Context) jobdata.Readiness { return stack.Ready(ctx) },
		Queue:        queue,
		Setup:        setup,
		ComfyVersion: comfy.New("").Version,
		Ports:        ports,
		Settings: webui.Settings{DatabaseURL: settings.DatabaseURL(), ComfyURL: settings.ComfyURL(), Slots: slots,
			RunLimit: time.Duration(limit) * time.Second, Broker: "Postgres",
			// The one mapping of a container path to a host path: the page
			// shows the path that the result of generate_image gives.
			Paths: opts.Queue.Paths},
	}))
	// Each request ends when ctx ends, so that a stream of events does not
	// hold the stop, and a call of /mcp cancels its job. The server has no
	// write time limit: a call of generate_image holds its POST open for
	// minutes.
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	fmt.Fprintf(out, "horus api: serves /mcp, the Horus UI, /readyz and /livez on http://%s\n", ln.Addr())
	if token == "" {
		fmt.Fprintf(out, "horus api: %s is not set, so /mcp refuses each request with 401. Set it in the .env of the stack, then run docker compose up -d\n", settings.APITokenVar)
	} else {
		fmt.Fprintf(out, "horus api: /mcp needs the bearer token of %s\n", settings.APITokenVar)
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	tick := time.NewTicker(readyEvery)
	defer tick.Stop()
	last := ""
	for {
		select {
		case err := <-errc:
			return err
		case <-tick.C:
			r := stack.Ready(ctx)
			if now := fmt.Sprintf("%d of %d, down: %v", r.Up, len(r.Checks), r.Down); now != last {
				fmt.Fprintf(out, "horus api: %s", r.Text())
				last = now
			}
			continue
		case <-ctx.Done():
		}
		break
	}
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(sctx)
	fmt.Fprintln(out, "horus api: stopped")
	return err
}

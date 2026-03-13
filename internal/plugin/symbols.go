// symbols.go exports the sdk package for the Yaegi interpreter.
//
// Yaegi loads Go source at runtime via reflection. When interpreted plugin
// code calls sdk.Documents.Read(), Yaegi resolves "sdk.Documents" through
// this symbol table. The table maps symbol names to reflect.Values pointing
// at the real sdk package globals, types, and constants.
//
// Interface wrappers (_sdk_DocumentStore, _sdk_TaskStore, etc.) exist because
// Yaegi cannot directly satisfy Go interfaces with interpreted types. Each
// wrapper struct has a WMethodName function field for every interface method.
// Yaegi populates these fields with the interpreted plugin's implementations,
// then the wrapper's concrete methods delegate to the fields. When an
// interface method is added or removed, the corresponding wrapper must be
// updated to match.

package plugin

import (
	"reflect"

	"github.com/traefik/yaegi/interp"

	"github.com/jpl-au/llmd/sdk"
)

// symbols returns Yaegi exports for the sdk package, wired to the
// adapter's own store fields rather than package-level globals. This
// means each adapter has isolated, request-scoped store access —
// Exec populates the fields before each plugin call so Yaegi reads
// the correct bridges for that request.
func (a *adapter) symbols() interp.Exports {
	return interp.Exports{
		"github.com/jpl-au/llmd/sdk/sdk": {
			// Domain stores — point at adapter fields, not package globals.
			"Documents":  reflect.ValueOf(&a.documents).Elem(),
			"Tasks":      reflect.ValueOf(&a.tasks).Elem(),
			"Links":      reflect.ValueOf(&a.links).Elem(),
			"Tags":       reflect.ValueOf(&a.tags).Elem(),
			"Activities": reflect.ValueOf(&a.activities).Elem(),
			"Mirror":     reflect.ValueOf(&a.mirror).Elem(),

			// Constants
			"GrepFull":     reflect.ValueOf(sdk.GrepFull),
			"GrepLines":    reflect.ValueOf(sdk.GrepLines),
			"GrepPaths":    reflect.ValueOf(sdk.GrepPaths),
			"GrepSections": reflect.ValueOf(sdk.GrepSections),
			"GrepSnippets": reflect.ValueOf(sdk.GrepSnippets),

			// Sentinel errors
			"ErrMissingArg": reflect.ValueOf(&sdk.ErrMissingArg).Elem(),
			"ErrInvalidArg": reflect.ValueOf(&sdk.ErrInvalidArg).Elem(),
			"ErrUnknownCmd": reflect.ValueOf(&sdk.ErrUnknownCmd).Elem(),
			"ErrNotFound":   reflect.ValueOf(&sdk.ErrNotFound).Elem(),
			"ErrNoSpec":     reflect.ValueOf(&sdk.ErrNoSpec).Elem(),
			"ErrExists":     reflect.ValueOf(&sdk.ErrExists).Elem(),

			// Functions
			"Init":      reflect.ValueOf(&sdk.Init).Elem(),
			"ParseArgs": reflect.ValueOf(sdk.ParseArgs),

			// Type definitions
			"Activity":        reflect.ValueOf((*sdk.Activity)(nil)),
			"Command":         reflect.ValueOf((*sdk.Command)(nil)),
			"Context":         reflect.ValueOf((*sdk.Context)(nil)),
			"Data":            reflect.ValueOf((*sdk.Data)(nil)),
			"Doc":             reflect.ValueOf((*sdk.Doc)(nil)),
			"ExportOpts":      reflect.ValueOf((*sdk.ExportOpts)(nil)),
			"ExportResult":    reflect.ValueOf((*sdk.ExportResult)(nil)),
			"Flag":            reflect.ValueOf((*sdk.Flag)(nil)),
			"FlagValues":      reflect.ValueOf((*sdk.FlagValues)(nil)),
			"GrepHit":         reflect.ValueOf((*sdk.GrepHit)(nil)),
			"GrepMode":        reflect.ValueOf((*sdk.GrepMode)(nil)),
			"GrepOpts":        reflect.ValueOf((*sdk.GrepOpts)(nil)),
			"ImportOpts":      reflect.ValueOf((*sdk.ImportOpts)(nil)),
			"ImportResult":    reflect.ValueOf((*sdk.ImportResult)(nil)),
			"Link":            reflect.ValueOf((*sdk.Link)(nil)),
			"ListOpts":        reflect.ValueOf((*sdk.ListOpts)(nil)),
			"Plugin":          reflect.ValueOf((*sdk.Plugin)(nil)),
			"Response":        reflect.ValueOf((*sdk.Response)(nil)),
			"Result":          reflect.ValueOf((*sdk.Result)(nil)),
			"Tag":             reflect.ValueOf((*sdk.Tag)(nil)),
			"TagInfo":         reflect.ValueOf((*sdk.TagInfo)(nil)),
			"Task":            reflect.ValueOf((*sdk.Task)(nil)),
			"TaskAddOpts":     reflect.ValueOf((*sdk.TaskAddOpts)(nil)),
			"TaskEvent":       reflect.ValueOf((*sdk.TaskEvent)(nil)),
			"TaskListOpts":    reflect.ValueOf((*sdk.TaskListOpts)(nil)),
			"TaskSetOpts":     reflect.ValueOf((*sdk.TaskSetOpts)(nil)),
			"StartOpts":       reflect.ValueOf((*sdk.StartOpts)(nil)),
			"StartBranchOpts": reflect.ValueOf((*sdk.StartBranchOpts)(nil)),
			"FinishOpts":      reflect.ValueOf((*sdk.FinishOpts)(nil)),
			"FinishResult":    reflect.ValueOf((*sdk.FinishResult)(nil)),
			"Text":            reflect.ValueOf((*sdk.Text)(nil)),
			"VacuumResult":    reflect.ValueOf((*sdk.VacuumResult)(nil)),
			"Version":         reflect.ValueOf((*sdk.Version)(nil)),

			// Interface definitions
			"DocumentStore": reflect.ValueOf((*sdk.DocumentStore)(nil)),
			"TaskStore":     reflect.ValueOf((*sdk.TaskStore)(nil)),
			"LinkStore":     reflect.ValueOf((*sdk.LinkStore)(nil)),
			"TagStore":      reflect.ValueOf((*sdk.TagStore)(nil)),
			"ActivityStore": reflect.ValueOf((*sdk.ActivityStore)(nil)),
			"MirrorStore":   reflect.ValueOf((*sdk.MirrorStore)(nil)),
			"PullResult":    reflect.ValueOf((*sdk.PullResult)(nil)),
			"PushOpts":      reflect.ValueOf((*sdk.PushOpts)(nil)),
			"PushResult":    reflect.ValueOf((*sdk.PushResult)(nil)),

			// Interface wrappers
			"_DocumentStore": reflect.ValueOf((*_sdk_DocumentStore)(nil)),
			"_TaskStore":     reflect.ValueOf((*_sdk_TaskStore)(nil)),
			"_LinkStore":     reflect.ValueOf((*_sdk_LinkStore)(nil)),
			"_TagStore":      reflect.ValueOf((*_sdk_TagStore)(nil)),
			"_ActivityStore": reflect.ValueOf((*_sdk_ActivityStore)(nil)),
			"_MirrorStore":   reflect.ValueOf((*_sdk_MirrorStore)(nil)),
			"_Plugin":        reflect.ValueOf((*_sdk_Plugin)(nil)),
			"_Response":      reflect.ValueOf((*_sdk_Response)(nil)),
		},
	}
}

// _sdk_Plugin is an interface wrapper for Plugin type
type _sdk_Plugin struct {
	IValue    any
	WCommands func() []sdk.Command
	WExec     func(ctx sdk.Context, cmd string, args []string) (sdk.Response, error)
	WName     func() string
}

func (W _sdk_Plugin) Commands() []sdk.Command {
	return W.WCommands()
}
func (W _sdk_Plugin) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
	return W.WExec(ctx, cmd, args)
}
func (W _sdk_Plugin) Name() string {
	return W.WName()
}

// _sdk_Response is an interface wrapper for Response type
type _sdk_Response struct {
	IValue    any
	WResponse func()
}

func (W _sdk_Response) Response() {
	W.WResponse()
}

// _sdk_DocumentStore is an interface wrapper for DocumentStore type
type _sdk_DocumentStore struct {
	IValue   any
	WRead    func(path string, version int) ([]byte, error)
	WWrite   func(path string, content []byte, author string, msg string) error
	WDelete  func(path string, author string) error
	WRestore func(path string, author string) error
	WMove    func(from string, to string, author string) error
	WList    func(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error)
	WExists  func(path string) (bool, error)
	WEdit    func(path string, old string, new string, author string, msg string) error
	WGlob    func(pattern string) ([]string, error)
	WGrep    func(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error)
	WHistory func(path string, limit int) ([]sdk.Version, error)
	WDiff    func(a string, b string, ctx int) (string, int, int, error)
	WRevert  func(path string, version int, author string, msg string) error
	WVacuum  func() (sdk.VacuumResult, error)
	WImport  func(dir string, opts sdk.ImportOpts) (*sdk.ImportResult, error)
	WExport  func(prefix string, dir string, opts sdk.ExportOpts) (*sdk.ExportResult, error)
	WPreview func(path string, lines int) (string, error)
}

func (W _sdk_DocumentStore) Read(path string, version int) ([]byte, error) {
	return W.WRead(path, version)
}
func (W _sdk_DocumentStore) Write(path string, content []byte, author string, msg string) error {
	return W.WWrite(path, content, author, msg)
}
func (W _sdk_DocumentStore) Delete(path string, author string) error {
	return W.WDelete(path, author)
}
func (W _sdk_DocumentStore) Restore(path string, author string) error {
	return W.WRestore(path, author)
}
func (W _sdk_DocumentStore) Move(from string, to string, author string) error {
	return W.WMove(from, to, author)
}
func (W _sdk_DocumentStore) List(prefix string, opts sdk.ListOpts) ([]sdk.Doc, error) {
	return W.WList(prefix, opts)
}
func (W _sdk_DocumentStore) Exists(path string) (bool, error) {
	return W.WExists(path)
}
func (W _sdk_DocumentStore) Edit(path string, old string, new string, author string, msg string) error {
	return W.WEdit(path, old, new, author, msg)
}
func (W _sdk_DocumentStore) Glob(pattern string) ([]string, error) {
	return W.WGlob(pattern)
}
func (W _sdk_DocumentStore) Grep(query string, opts sdk.GrepOpts) ([]sdk.GrepHit, error) {
	return W.WGrep(query, opts)
}
func (W _sdk_DocumentStore) History(path string, limit int) ([]sdk.Version, error) {
	return W.WHistory(path, limit)
}
func (W _sdk_DocumentStore) Diff(a string, b string, ctx int) (string, int, int, error) {
	return W.WDiff(a, b, ctx)
}
func (W _sdk_DocumentStore) Revert(path string, version int, author string, msg string) error {
	return W.WRevert(path, version, author, msg)
}
func (W _sdk_DocumentStore) Vacuum() (sdk.VacuumResult, error) {
	return W.WVacuum()
}
func (W _sdk_DocumentStore) Import(dir string, opts sdk.ImportOpts) (*sdk.ImportResult, error) {
	return W.WImport(dir, opts)
}
func (W _sdk_DocumentStore) Export(prefix string, dir string, opts sdk.ExportOpts) (*sdk.ExportResult, error) {
	return W.WExport(prefix, dir, opts)
}
func (W _sdk_DocumentStore) Preview(path string, lines int) (string, error) {
	return W.WPreview(path, lines)
}

// _sdk_TaskStore is an interface wrapper for TaskStore type
type _sdk_TaskStore struct {
	IValue        any
	WAdd          func(title string, body []byte, opts sdk.TaskAddOpts) (*sdk.Task, error)
	WRead         func(key string) (*sdk.Task, error)
	WList         func(opts sdk.TaskListOpts) ([]*sdk.Task, error)
	WMove         func(key string, column string, author string) error
	WSet          func(key string, author string, opts sdk.TaskSetOpts) error
	WDelete       func(key string, author string) (*sdk.Task, error)
	WRestore      func(key string, author string) (*sdk.Task, error)
	WColumns      func() ([]string, error)
	WAddColumn    func(name string, after string, author string) error
	WRemoveColumn func(name string, author string) error
	WMoveColumn   func(name string, after string, author string) error
	WStart        func(key string, author string, opts sdk.StartOpts) (*sdk.Task, error)
	WStartBranch  func(key string, author string, opts sdk.StartBranchOpts) (*sdk.Task, error)
	WFinish       func(key string, author string, opts sdk.FinishOpts) (*sdk.FinishResult, error)
	WByBranch     func(branch string) (*sdk.Task, error)
	WCheckSpecs   func(tasks []*sdk.Task) (map[string]bool, error)
	WLink         func(key string, target string, author string) error
	WLinks        func(key string, dir string) ([]sdk.Link, error)
	WLog          func(key string, limit int) ([]sdk.TaskEvent, error)
}

func (W _sdk_TaskStore) Add(title string, body []byte, opts sdk.TaskAddOpts) (*sdk.Task, error) {
	return W.WAdd(title, body, opts)
}
func (W _sdk_TaskStore) Read(key string) (*sdk.Task, error) {
	return W.WRead(key)
}
func (W _sdk_TaskStore) List(opts sdk.TaskListOpts) ([]*sdk.Task, error) {
	return W.WList(opts)
}
func (W _sdk_TaskStore) Move(key string, column string, author string) error {
	return W.WMove(key, column, author)
}
func (W _sdk_TaskStore) Set(key string, author string, opts sdk.TaskSetOpts) error {
	return W.WSet(key, author, opts)
}
func (W _sdk_TaskStore) Delete(key string, author string) (*sdk.Task, error) {
	return W.WDelete(key, author)
}
func (W _sdk_TaskStore) Restore(key string, author string) (*sdk.Task, error) {
	return W.WRestore(key, author)
}
func (W _sdk_TaskStore) Columns() ([]string, error) {
	return W.WColumns()
}
func (W _sdk_TaskStore) AddColumn(name string, after string, author string) error {
	return W.WAddColumn(name, after, author)
}
func (W _sdk_TaskStore) RemoveColumn(name string, author string) error {
	return W.WRemoveColumn(name, author)
}
func (W _sdk_TaskStore) MoveColumn(name string, after string, author string) error {
	return W.WMoveColumn(name, after, author)
}
func (W _sdk_TaskStore) Start(key string, author string, opts sdk.StartOpts) (*sdk.Task, error) {
	return W.WStart(key, author, opts)
}
func (W _sdk_TaskStore) StartBranch(key string, author string, opts sdk.StartBranchOpts) (*sdk.Task, error) {
	return W.WStartBranch(key, author, opts)
}
func (W _sdk_TaskStore) Finish(key string, author string, opts sdk.FinishOpts) (*sdk.FinishResult, error) {
	return W.WFinish(key, author, opts)
}
func (W _sdk_TaskStore) ByBranch(branch string) (*sdk.Task, error) {
	return W.WByBranch(branch)
}
func (W _sdk_TaskStore) CheckSpecs(tasks []*sdk.Task) (map[string]bool, error) {
	return W.WCheckSpecs(tasks)
}
func (W _sdk_TaskStore) Link(key string, target string, author string) error {
	return W.WLink(key, target, author)
}
func (W _sdk_TaskStore) Links(key string, dir string) ([]sdk.Link, error) {
	return W.WLinks(key, dir)
}
func (W _sdk_TaskStore) Log(key string, limit int) ([]sdk.TaskEvent, error) {
	return W.WLog(key, limit)
}

// _sdk_LinkStore is an interface wrapper for LinkStore type
type _sdk_LinkStore struct {
	IValue  any
	WAdd    func(from string, to string, label string, author string) error
	WRemove func(from string, to string, author string) error
	WList   func(path string, dir string) ([]sdk.Link, error)
}

func (W _sdk_LinkStore) Add(from string, to string, label string, author string) error {
	return W.WAdd(from, to, label, author)
}
func (W _sdk_LinkStore) Remove(from string, to string, author string) error {
	return W.WRemove(from, to, author)
}
func (W _sdk_LinkStore) List(path string, dir string) ([]sdk.Link, error) {
	return W.WList(path, dir)
}

// _sdk_TagStore is an interface wrapper for TagStore type
type _sdk_TagStore struct {
	IValue  any
	WAdd    func(path string, name string, author string) error
	WRemove func(path string, name string, author string) error
	WList   func(path string) ([]sdk.Tag, error)
	WAll    func() ([]sdk.TagInfo, error)
	WFind   func(name string) ([]string, error)
}

func (W _sdk_TagStore) Add(path string, name string, author string) error {
	return W.WAdd(path, name, author)
}
func (W _sdk_TagStore) Remove(path string, name string, author string) error {
	return W.WRemove(path, name, author)
}
func (W _sdk_TagStore) List(path string) ([]sdk.Tag, error) {
	return W.WList(path)
}
func (W _sdk_TagStore) All() ([]sdk.TagInfo, error) {
	return W.WAll()
}
func (W _sdk_TagStore) Find(name string) ([]string, error) {
	return W.WFind(name)
}

// _sdk_ActivityStore is an interface wrapper for ActivityStore type
type _sdk_ActivityStore struct {
	IValue  any
	WRecent func(limit int) ([]sdk.Activity, error)
}

func (W _sdk_ActivityStore) Recent(limit int) ([]sdk.Activity, error) {
	return W.WRecent(limit)
}

// _sdk_MirrorStore is an interface wrapper for MirrorStore type
type _sdk_MirrorStore struct {
	IValue     any
	WDirectory func() string
	WPull      func(prefix string, dir string) (*sdk.PullResult, error)
	WPush      func(dir string, opts sdk.PushOpts) (*sdk.PushResult, error)
}

func (W _sdk_MirrorStore) Directory() string {
	return W.WDirectory()
}
func (W _sdk_MirrorStore) Pull(prefix string, dir string) (*sdk.PullResult, error) {
	return W.WPull(prefix, dir)
}
func (W _sdk_MirrorStore) Push(dir string, opts sdk.PushOpts) (*sdk.PushResult, error) {
	return W.WPush(dir, opts)
}

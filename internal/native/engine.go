//go:build (windows && amd64) || (darwin && arm64)

package native

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Each platform's file embeds the engine as libCompressed, DEFLATE-compressed
// by tools/pack.go from the Chromium build output, names it libName, and
// provides openLibrary and call. Content-addressed on extraction, so go.sum
// covers integrity.

// ─── C ABI structs (mirror wrc_net.h; same layout on amd64 and arm64) ────────

type wrcBytes struct {
	p uintptr
	n uintptr
}

type wrcField struct {
	name  wrcBytes
	value wrcBytes
}

type wrcRequestOpts struct {
	structSize  uintptr
	method      wrcBytes
	url         wrcBytes
	fields      uintptr
	fieldCount  uintptr
	bodyMode    int32
	_           int32
	body        uintptr
	bodyLen     uintptr
	priority    int32
	prioritySet int32
	incremental int32
	_           int32
	partition   wrcBytes
	credentials int32
	newConn     int32
	userTag     uint64
}

type wrcEvent struct {
	typ      int32
	netError int32
	userTag  uint64
}

const (
	bodyNone   = 0
	bodyBytes  = 1
	bodyStream = 2

	evHeaders       = 1
	evReadable      = 2
	evError         = 3
	evInformational = 4

	wrcPending = -1
)

// ─── Loaded procs ────────────────────────────────────────────────────────────

// proc is the address of one of the engine's functions.
type proc uintptr

// Call calls the function with integer and pointer arguments; the result is
// the full return register, so callers narrow int results themselves. Like
// syscall.Proc.Call, pointers converted to uintptr in the call's arguments
// stay on the heap and alive until it returns.
//
//go:uintptrescapes
func (f proc) Call(args ...uintptr) (uintptr, uintptr, error) {
	return call(uintptr(f), args...), 0, nil
}

type procs struct {
	initFn        proc
	version       proc
	errorName     proc
	free          proc
	clientNew     proc
	setProxyAuth  proc
	clientClose   proc
	requestStart  proc
	requestWrite  proc
	requestFinish proc
	requestStatus proc
	requestFields proc
	requestConn   proc
	requestRead   proc
	requestCancel proc
	requestFree   proc
	poll          proc
	wakeup        proc
}

var (
	p        *procs
	initOnce sync.Once
	initErr  error

	registry   sync.Map // uint64 -> *Req
	tagCounter atomic.Uint64
)

// Init loads the engine and performs one-time process setup.
func Init(configJSON string) error {
	initOnce.Do(func() {
		initErr = doInit(configJSON)
	})
	return initErr
}

func doInit(configJSON string) error {
	path, err := extractLibrary()
	if err != nil {
		return err
	}
	lookup, err := openLibrary(path)
	if err != nil {
		return err
	}
	find := func(name string) proc {
		addr, e := lookup(name)
		if e != nil && err == nil {
			err = e
		}
		return proc(addr)
	}
	p = &procs{
		initFn:        find("wrc_init"),
		version:       find("wrc_version"),
		errorName:     find("wrc_error_name"),
		free:          find("wrc_free"),
		clientNew:     find("wrc_client_new"),
		setProxyAuth:  find("wrc_client_set_proxy_auth"),
		clientClose:   find("wrc_client_close"),
		requestStart:  find("wrc_request_start"),
		requestWrite:  find("wrc_request_write"),
		requestFinish: find("wrc_request_finish"),
		requestStatus: find("wrc_request_status"),
		requestFields: find("wrc_request_fields"),
		requestConn:   find("wrc_request_conn_json"),
		requestRead:   find("wrc_request_read"),
		requestCancel: find("wrc_request_cancel"),
		requestFree:   find("wrc_request_free"),
		poll:          find("wrc_poll"),
		wakeup:        find("wrc_wakeup"),
	}
	if err != nil {
		return err
	}

	cfg := append([]byte(configJSON), 0)
	rc, _, _ := p.initFn.Call(uintptr(unsafe.Pointer(&cfg[0])))
	if int32(rc) != 0 {
		return &NetError{Code: int32ToInt(rc), Name: errorName(int32ToInt(rc))}
	}

	go pollLoop()
	return nil
}

// extractLibrary decompresses the embedded engine into a content-addressed
// cache directory and returns its path, writing it only if missing or altered.
func extractLibrary() (string, error) {
	zr := flate.NewReader(bytes.NewReader(libCompressed))
	raw, err := io.ReadAll(zr)
	_ = zr.Close()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	hexSum := hex.EncodeToString(sum[:])

	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "browserscale", "net", hexSum)
	path := filepath.Join(dir, libName)

	if existing, e := os.ReadFile(path); e == nil {
		if s := sha256.Sum256(existing); s == sum {
			return path, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, "wrc_net-*.tmp")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		// Another process may have won the race; accept an already-correct file.
		if existing, e := os.ReadFile(path); e == nil {
			if s := sha256.Sum256(existing); s == sum {
				return path, nil
			}
		}
		return "", err
	}
	return path, nil
}

func Version() string {
	if p == nil {
		return ""
	}
	r, _, _ := p.version.Call()
	return cstr(r)
}

func NewClient(configJSON string) (uintptr, error) {
	cfg := append([]byte(configJSON), 0)
	var errPtr uintptr
	client, _, _ := p.clientNew.Call(
		uintptr(unsafe.Pointer(&cfg[0])),
		uintptr(unsafe.Pointer(&errPtr)))
	if client == 0 {
		msg := cstr(errPtr)
		if errPtr != 0 {
			p.free.Call(errPtr)
		}
		if msg == "" {
			msg = "wrc_client_new failed"
		}
		return 0, &clientError{msg}
	}
	return client, nil
}

type clientError struct{ msg string }

func (e *clientError) Error() string { return "browserscale-net: " + e.msg }

func SetProxyAuth(client uintptr, user, pass string) error {
	ub := []byte(user)
	pb := []byte(pass)
	rc := setProxyAuth(client, bytesOf(ub), bytesOf(pb))
	keepAlive(ub, pb)
	if int32(rc) != 0 {
		return &NetError{Code: int32ToInt(rc), Name: errorName(int32ToInt(rc))}
	}
	return nil
}

func CloseClient(client uintptr) {
	if client != 0 {
		p.clientClose.Call(client)
	}
}

// ─── Request ─────────────────────────────────────────────────────────────────

// Req is one in-flight request. Methods are safe for a single owning goroutine;
// the dispatcher goroutine only signals channels.
type Req struct {
	handle uintptr
	tag    uint64

	mu          sync.Mutex
	err         error
	headersDone bool

	headersCh  chan struct{}
	readableCh chan struct{}
}

func Start(client uintptr, opts StartOpts) (*Req, error) {
	r := &Req{
		tag:        tagCounter.Add(1),
		headersCh:  make(chan struct{}),
		readableCh: make(chan struct{}, 1),
	}
	registry.Store(r.tag, r)

	// Keep all backing byte slices alive until the call returns; the C side
	// copies them synchronously.
	method := []byte(opts.Method)
	url := []byte(opts.URL)
	part := []byte(opts.Partition)

	var keep [][]byte
	keep = append(keep, method, url, part)

	cFields := make([]wrcField, len(opts.Fields))
	for i, f := range opts.Fields {
		nb := []byte(f.Name)
		vb := []byte(f.Value)
		keep = append(keep, nb, vb)
		cFields[i] = wrcField{name: bytesOf(nb), value: bytesOf(vb)}
	}

	co := wrcRequestOpts{
		method:     bytesOf(method),
		url:        bytesOf(url),
		partition:  bytesOf(part),
		fieldCount: uintptr(len(cFields)),
	}
	co.structSize = unsafe.Sizeof(co)
	if len(cFields) > 0 {
		co.fields = uintptr(unsafe.Pointer(&cFields[0]))
	}
	if opts.PrioritySet {
		co.priority = int32(opts.Priority)
		co.prioritySet = 1
	}
	if opts.Incremental {
		co.incremental = 1
	}
	if opts.Credentials {
		co.credentials = 1
	}
	switch {
	case opts.Stream:
		co.bodyMode = bodyStream
	case len(opts.Body) > 0:
		co.bodyMode = bodyBytes
		co.body = uintptr(unsafe.Pointer(&opts.Body[0]))
		co.bodyLen = uintptr(len(opts.Body))
		keep = append(keep, opts.Body)
	default:
		co.bodyMode = bodyNone
	}
	co.userTag = r.tag

	var out uintptr
	rc, _, _ := p.requestStart.Call(client,
		uintptr(unsafe.Pointer(&co)),
		uintptr(unsafe.Pointer(&out)))
	keepAlive(keep...)
	keepSlices(cFields)

	if int32(rc) != 0 || out == 0 {
		registry.Delete(r.tag)
		code := int32ToInt(rc)
		return nil, &NetError{Code: code, Name: errorName(code)}
	}
	r.handle = out
	return r, nil
}

func (r *Req) Headers(ctx context.Context) (status int, proto string, fields []HeaderField, connJSON string, err error) {
	select {
	case <-r.headersCh:
	case <-ctx.Done():
		r.Cancel()
		return 0, "", nil, "", ctx.Err()
	}
	r.mu.Lock()
	e := r.err
	r.mu.Unlock()
	if e != nil {
		return 0, "", nil, "", e
	}

	var code int32
	var ver wrcBytes
	p.requestStatus.Call(r.handle,
		uintptr(unsafe.Pointer(&code)),
		uintptr(unsafe.Pointer(&ver)))
	status = int(code)
	proto = goBytes(ver)

	var fptr uintptr
	var fcount uintptr
	p.requestFields.Call(r.handle,
		uintptr(unsafe.Pointer(&fptr)),
		uintptr(unsafe.Pointer(&fcount)))
	fields = make([]HeaderField, 0, fcount)
	sz := unsafe.Sizeof(wrcField{})
	for i := uintptr(0); i < fcount; i++ {
		f := (*wrcField)(unsafe.Pointer(fptr + i*sz))
		fields = append(fields, HeaderField{Name: goBytes(f.name), Value: goBytes(f.value)})
	}

	cj, _, _ := p.requestConn.Call(r.handle)
	connJSON = cstr(cj)
	if cj != 0 {
		p.free.Call(cj)
	}
	return status, proto, fields, connJSON, nil
}

func (r *Req) Read(ctx context.Context, buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	for {
		got, _, _ := p.requestRead.Call(r.handle,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)))
		n := int32(uint32(got))
		switch {
		case n > 0:
			return int(n), nil
		case n == 0:
			return 0, io.EOF
		case n == wrcPending:
			r.mu.Lock()
			e := r.err
			r.mu.Unlock()
			if e != nil {
				return 0, e
			}
			select {
			case <-r.readableCh:
			case <-ctx.Done():
				r.Cancel()
				return 0, ctx.Err()
			}
		default:
			code := int(n)
			return 0, &NetError{Code: code, Name: errorName(code)}
		}
	}
}

func (r *Req) Write(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	rc, _, _ := p.requestWrite.Call(r.handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)))
	keepAlive(buf)
	if int32(rc) != 0 {
		return &NetError{Code: int32ToInt(rc), Name: errorName(int32ToInt(rc))}
	}
	return nil
}

func (r *Req) Finish() error {
	rc, _, _ := p.requestFinish.Call(r.handle)
	if int32(rc) != 0 {
		return &NetError{Code: int32ToInt(rc), Name: errorName(int32ToInt(rc))}
	}
	return nil
}

func (r *Req) Cancel() {
	if r.handle != 0 {
		p.requestCancel.Call(r.handle)
	}
}

func (r *Req) Free() {
	registry.Delete(r.tag)
	if r.handle != 0 {
		p.requestFree.Call(r.handle)
		r.handle = 0
	}
}

// ─── Dispatcher ──────────────────────────────────────────────────────────────

func pollLoop() {
	const batch = 64
	evs := make([]wrcEvent, batch)
	for {
		n, _, _ := p.poll.Call(
			uintptr(unsafe.Pointer(&evs[0])),
			uintptr(batch),
			uintptr(^uintptr(0))) // -1: block
		count := int(n)
		for i := 0; i < count; i++ {
			dispatch(evs[i])
		}
	}
}

func dispatch(ev wrcEvent) {
	v, ok := registry.Load(ev.userTag)
	if !ok {
		return
	}
	r := v.(*Req)
	switch ev.typ {
	case evHeaders:
		r.closeHeaders()
	case evInformational:
		// Ignored for now; could be surfaced as a 1xx callback later.
	case evError:
		r.mu.Lock()
		if r.err == nil {
			code := int(ev.netError)
			r.err = &NetError{Code: code, Name: errorName(code)}
		}
		r.mu.Unlock()
		r.closeHeaders()
		r.signalReadable()
	case evReadable:
		if ev.netError < 0 {
			r.mu.Lock()
			if r.err == nil {
				code := int(ev.netError)
				r.err = &NetError{Code: code, Name: errorName(code)}
			}
			r.mu.Unlock()
		}
		r.signalReadable()
	}
}

func (r *Req) closeHeaders() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.headersDone {
		r.headersDone = true
		close(r.headersCh)
	}
}

func (r *Req) signalReadable() {
	select {
	case r.readableCh <- struct{}{}:
	default:
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func errorName(code int) string {
	if p == nil || p.errorName == 0 {
		return ""
	}
	r, _, _ := p.errorName.Call(uintptr(code))
	return cstr(r)
}

func bytesOf(b []byte) wrcBytes {
	if len(b) == 0 {
		return wrcBytes{}
	}
	return wrcBytes{p: uintptr(unsafe.Pointer(&b[0])), n: uintptr(len(b))}
}

func goBytes(b wrcBytes) string {
	if b.p == 0 || b.n == 0 {
		return ""
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(b.p)), int(b.n)))
}

func cstr(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var out []byte
	for i := 0; ; i++ {
		c := *(*byte)(unsafe.Pointer(ptr + uintptr(i)))
		if c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

func int32ToInt(r uintptr) int { return int(int32(uint32(r))) }

// keepAlive and keepSlices prevent the GC from freeing backing memory while the
// C call that referenced it is in progress.
func keepAlive(bs ...[]byte) {
	runtime.KeepAlive(bs)
}

func keepSlices(f []wrcField) {
	runtime.KeepAlive(f)
}

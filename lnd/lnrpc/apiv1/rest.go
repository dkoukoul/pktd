package apiv1

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/mux"

	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/lock"
	"github.com/pkt-cash/pktd/btcutil/util"
	"github.com/pkt-cash/pktd/generated/pkthelp"
	"github.com/pkt-cash/pktd/generated/proto/restrpc_pb/help_pb"
	"github.com/pkt-cash/pktd/generated/proto/rpc_pb"
	"github.com/pkt-cash/pktd/pktlog/log"
)

const _api_v1_ = "/api/v1/"

//const _help = "help"

// Functions

func respondError(res http.ResponseWriter, code int, msg string) er.R {
	res.Header().Set("Content-Type", "text/plain")
	http.Error(res, msg, code)
	return nil
}

func parentCat(cat string) string {
	idx := strings.LastIndex(cat, "/")
	if idx < 0 {
		return ""
	}
	return cat[:idx]
}

func categorize(
	out *help_pb.Category,
	shortDesc string,
	path []string,
	fpath []string,
	cats map[string][]string,
) {
	if len(path) < 1 {
		panic("path empty")
	}
	fpath = append(fpath, path[0])
	var cat *help_pb.Category
	if c, ok := out.Categories[path[0]]; ok {
		cat = c
	} else {
		desc := []string{"TODO"}
		if d, ok := cats[strings.Join(fpath, "/")]; ok {
			desc = d
		}
		cat = &help_pb.Category{
			Description: append(desc, strings.Join(fpath, "/")),
			Categories:  make(map[string]*help_pb.Category),
			Endpoints:   make(map[string]string),
		}
		if out.Categories == nil {
			out.Categories = make(map[string]*help_pb.Category)
		}
		out.Categories[path[0]] = cat
	}
	if len(path) > 1 {
		categorize(cat, shortDesc, path[1:], fpath, cats)
	} else {
		cat.Endpoints[path[0]] = shortDesc
	}
}

func typeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

func toPm[T proto.Message]() proto.Message {
	return reflect.New(typeOf[T]()).Elem().Interface().(proto.Message)
}

//	convert	pkthelp.type to REST help proto struct
func convertHelpType(t pkthelp.Type) *help_pb.Type {
	resultType := &help_pb.Type{
		Name:        t.Name,
		Description: t.Description,
	}

	//	convert the array of fields
	for _, field := range t.Fields {
		resultType.Fields = append(resultType.Fields, &help_pb.Field{
			Name:        field.Name,
			Description: field.Description,
			Repeated:    field.Repeated,
			Type:        convertHelpType(field.Type),
		})
	}

	return resultType
}

func trimSplit(s string) []string {
	return util.Filter(
		util.Map(strings.Split(s, "\n"), strings.TrimSpace),
		func(s string) bool { return s != "" },
	)
}

func unmarshal(r *http.Request, m proto.Message, isJson bool) er.R {
	if isJson {
		if err := jsonpb.Unmarshal(r.Body, m); err != nil {
			return er.E(err)
		}
	} else {
		if b, err := io.ReadAll(r.Body); err != nil {
			return er.E(err)
		} else if err := proto.Unmarshal(b, m); err != nil {
			return er.E(err)
		}
	}
	return nil
}
func marshal(w http.ResponseWriter, m proto.Message, isJson bool) er.R {
	if m == nil {
		return nil
	}
	if isJson {
		marshaler := jsonpb.Marshaler{
			OrigName:     false,
			EnumsAsInts:  false,
			EmitDefaults: true,
			Indent:       "\t",
		}
		if s, err := marshaler.MarshalToString(m); err != nil {
			return er.E(err)
		} else if _, err := io.WriteString(w, s); err != nil {
			return er.E(err)
		}
	} else {
		if b, err := proto.Marshal(m); err != nil {
			return er.E(err)
		} else if _, err := w.Write(b); err != nil {
			return er.E(err)
		}
	}
	return nil
}

// Endpoint

type endpoint struct {
	path     string
	category string
	mkReq    func() proto.Message
	helpRes  help_pb.EndpointHelp
	f        func(m proto.Message) (proto.Message, er.R)
}

func (e *endpoint) serveHttpOrErr(w http.ResponseWriter, r *http.Request, isJson bool) er.R {

	//	command URI handler
	req := e.mkReq()

	if r.Method != "POST" && r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return er.New("405 - Method not allowed: " + r.Method)
	}

	// There is actually a req struct, it's not Null.
	if _, ok := req.(*rpc_pb.Null); !ok {
		//	check the method again, because if it's a GET, there's no request payload
		if r.Method == "POST" {
			if err := unmarshal(r, req, isJson); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return err
			}
		} else if !util.Contains(e.helpRes.Features, help_pb.F_ALLOW_GET) {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return er.New("405 - Request should be a POST because the endpoint requires input")
		}
	}
	if res, err := e.f(req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	} else if err := marshal(w, res, isJson); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func (e *endpoint) serveHTTP(w http.ResponseWriter, r *http.Request) er.R {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	isJson := strings.Contains(ct, "application/json")
	if !isJson && !strings.Contains(ct, "application/protobuf") {
		if r.Method == "GET" {
			isJson = true
		} else {
			w.Header().Set("Connection", "close")
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, "415 - Invalid content type, must be json or protobuf", http.StatusUnsupportedMediaType)
			return nil
		}
	}
	if err := e.serveHttpOrErr(w, r, isJson); err != nil {
		if err = marshal(w, &rpc_pb.RestError{
			Message: err.Message(),
			Stack:   err.Stack(),
		}, isJson); err != nil {
			log.Errorf("Error replying to request for [%s] from [%s] - error sending error, giving up: [%s]",
				r.RequestURI, r.RemoteAddr, err)
		}
	}
	return nil
}

func (e *endpoint) respondHelp(res http.ResponseWriter, r *http.Request) er.R {
	he := endpoint{
		path:  e.path,
		mkReq: toPm[*rpc_pb.Null],
		f: func(m proto.Message) (proto.Message, er.R) {
			return &e.helpRes, nil
		},
	}
	return he.serveHTTP(res, r)
}

// Apiv1

type apiInt struct {
	cats  lock.GenRwLock[map[string][]string]
	funcs lock.GenRwLock[map[string]*endpoint]
}

type Apiv1 struct {
	internal *apiInt
	category string
}

func (a *Apiv1) cat(path string, description *string) *Apiv1 {
	if len(path) == 0 {
		panic("Cannot define category with no path")
	}
	if path[0] == '/' {
		panic("Path must not begin with a /")
	}
	if path[len(path)-1] == '/' {
		panic("Path must not end with a /")
	}
	if a.category != "" {
		path = a.category + "/" + path
	}
	a.internal.cats.W().In(func(cats *map[string][]string) er.R {
		if description == nil {
			if _, ok := (*cats)[path]; !ok {
				log.Warnf("No such defined category [%s]", path)
			}
		} else {
			if _, ok := (*cats)[path]; ok {
				log.Warnf("Creating category [%s] but it already exists")
			}
			(*cats)[path] = trimSplit(*description)
		}
		return nil
	})
	return &Apiv1{
		internal: a.internal,
		category: path,
	}
}

func (a *Apiv1) Category(path string) *Apiv1 {
	return a.cat(path, nil)
}

func DefineCategory(
	a *Apiv1,
	path string,
	description string,
) *Apiv1 {
	return a.cat(path, &description)
}

func Endpoint[Q proto.Message, R proto.Message](
	a *Apiv1,
	name string,
	description string,
	f func(req Q) (R, er.R),
	features ...help_pb.F,
) {
	path := name
	if a.category != "" {
		if path != "" {
			path = a.category + "/" + path
		} else {
			path = a.category
		}
	}

	// If no stability, set stability to EXPERIMENTAL
	hasStability := false
	for _, f := range features {
		switch f {
		case help_pb.F_DEPRECATED:
			fallthrough
		case help_pb.F_EXPERIMENTAL:
			fallthrough
		case help_pb.F_STABLE:
			fallthrough
		case help_pb.F_UNSTABLE:
			if hasStability {
				log.Warnf("Error registering RPC [%s]: [%s]", path,
					"Stability has been specified more than once")
			}
			hasStability = true
		}
	}
	if !hasStability {
		// No defined stability = experimental
		features = append(features, help_pb.F_EXPERIMENTAL)
	}

	// We're not going to return an error from here because
	// nobody wants to handle runtime errors while setting up
	// RPCs, in principle it should just panic, but we'll be nice
	// and print a warning only.
	log.Infof("Registering RPC [%s]", path)
	req := toPm[Q]()
	reqHt, err := pkthelp.Help(req)
	if err != nil {
		log.Warnf("Error registering RPC [%s]: [%s]", path, err)
		return
	}
	res := toPm[R]()
	resHt, err := pkthelp.Help(res)
	if err != nil {
		log.Warnf("Error registering RPC [%s]: [%s]", path, err)
		return
	}
	a.internal.funcs.W().In(func(funcs *map[string]*endpoint) er.R {
		(*funcs)[_api_v1_+path] = &endpoint{
			path:     _api_v1_ + path,
			mkReq:    toPm[Q],
			category: a.category,
			helpRes: help_pb.EndpointHelp{
				Path:        _api_v1_ + path,
				Description: trimSplit(description),
				Request:     convertHelpType(reqHt),
				Response:    convertHelpType(resHt),
				Features:    features,
			},
			f: func(m proto.Message) (proto.Message, er.R) {
				if query, ok := m.(Q); !ok {
					panic("invalid type")
				} else {
					return f(query)
				}
			},
		}
		return nil
	})
}

func Deregister(a *Apiv1, path string) er.R {
	return a.internal.funcs.W().In(func(eps *map[string]*endpoint) er.R {
		_, ok := (*eps)[path]
		if !ok {
			return er.New("not found")
		}
		delete(*eps, path)
		return nil
	})
}

type epInfo struct {
	shortDesc string
	path      string
}

func (a *Apiv1) masterHelp() (*help_pb.Category, er.R) {
	var eps []epInfo
	a.internal.funcs.R().In(func(t *map[string]*endpoint) er.R {
		for _, ep := range *t {
			eps = append(eps, epInfo{
				shortDesc: util.Iff(
					len(ep.helpRes.Description) > 0,
					func() string { return ep.helpRes.Description[0] },
					"<UNDEFINED>",
				),
				cat: ep.category,
			})
		}
		return nil
	})
	cats := make(map[string][]string)
	a.internal.cats.R().In(func(t *map[string][]string) er.R {
		for k, v := range *t {
			cats[k] = v
		}
		return nil
	})
	res := help_pb.Category{
		Description: []string{"PLD - The PKT Lightning Daemon REST interface"},
		Categories:  make(map[string]*help_pb.Category),
		Endpoints:   make(map[string]string),
	}
	for _, epi := range eps {
		path := util.Filter(strings.Split(epi.cat, "/"), func(s string) bool { return s != "" })
		log.Infof("Endpoint: %v", path)
		categorize(&res, epi.shortDesc, path, []string{}, cats)
	}
	return &res, nil
}

func New() (*Apiv1, *mux.Router) {
	r := mux.NewRouter()
	out := Apiv1{
		internal: &apiInt{
			funcs: lock.NewGenRwLock(make(map[string]*endpoint), "apiv1.funcs"),
			cats:  lock.NewGenRwLock(make(map[string][]string), "apiv1.cats"),
		},
	}

	//	Handle everything in the 404 handler, because this allows us to *remove* endpoints.
	r.NotFoundHandler = http.HandlerFunc(func(res http.ResponseWriter, r *http.Request) {
		if strings.Index(r.URL.Path, _api_v1_+"help/") == 0 {
			// handle this as a help req
			p := strings.Replace(r.URL.Path, _api_v1_+"help/", _api_v1_, 1)
			if err := out.internal.funcs.R().In(func(funcs *map[string]*endpoint) er.R {
				if ep, ok := (*funcs)[p]; ok {
					return ep.respondHelp(res, r)
				}
				return respondError(res,
					404,
					fmt.Sprintf(
						"No such help endpoint [%s], use /api/v1/help to see the full list",
						r.URL.Path,
					),
				)
			}); err != nil {
				log.Warnf("Error responding to RPC req: [%s]", err)
			}
		} else {
			if err := out.internal.funcs.R().In(func(funcs *map[string]*endpoint) er.R {
				if ep, ok := (*funcs)[r.URL.Path]; ok {
					return ep.serveHTTP(res, r)
				}
				return respondError(res,
					404,
					fmt.Sprintf(
						"No such endpoint [%s], use /api/v1/help to see the full list",
						r.URL.Path,
					),
				)
			}); err != nil {
				log.Warnf("Error responding to RPC req: [%s]", err)
			}
		}
	})

	//	add a handler for websocket endpoint
	r.Handle(_api_v1_+"meta/websocket", http.HandlerFunc(func(httpResponse http.ResponseWriter, httpRequest *http.Request) {
		// webSocketHandler(&out, httpResponse, httpRequest) TODO
	}))

	// Finally, register the master help endpoint
	Endpoint(
		&out,
		"help",
		`
		Output an index of RPC functions which can be called
		
		This is an internal endpoint which provides a manifest of all registered API endpoints in pld.
		`,
		func(_ *rpc_pb.Null) (*help_pb.Category, er.R) {
			return out.masterHelp()
		},
	)

	return &out, r
}

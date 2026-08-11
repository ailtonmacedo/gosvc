package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func compileGeneratedProject(t *testing.T, destination string, postgres bool, production ...bool) {
	t.Helper()
	isProduction := len(production) > 0 && production[0]
	isDistributed := len(production) > 1 && production[1]
	stubs := filepath.Join(t.TempDir(), "stubs")
	chiDir := filepath.Join(stubs, "chi")
	writeTestFile(t, filepath.Join(chiDir, "go.mod"), "module github.com/go-chi/chi/v5\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(chiDir, "chi.go"), `package chi
import("context";"net/http";"strings")
type Router interface{http.Handler;Use(...func(http.Handler)http.Handler);Get(string,http.HandlerFunc);Post(string,http.HandlerFunc);Put(string,http.HandlerFunc);Delete(string,http.HandlerFunc);Route(string,func(Router));Group(func(Router))}
type Mux struct{ mux *http.ServeMux; middleware []func(http.Handler) http.Handler; prefix string }
func NewRouter() *Mux { return &Mux{mux:http.NewServeMux()} }
func (m *Mux) Use(values ...func(http.Handler) http.Handler){ m.middleware=append(m.middleware, values...) }
func (m *Mux) handle(method,pattern string,handler http.HandlerFunc){full:=m.prefix+pattern;m.mux.HandleFunc(method+" "+full,func(w http.ResponseWriter,r *http.Request){if strings.Contains(full,"{id}"){parts:=strings.Split(strings.Trim(r.URL.Path,"/"),"/");if len(parts)>0{r=r.WithContext(context.WithValue(r.Context(),paramKey("id"),parts[len(parts)-1]))}};handler(w,r)})}
func (m *Mux) Get(p string,h http.HandlerFunc){m.handle("GET",p,h)}
func (m *Mux) Post(p string,h http.HandlerFunc){m.handle("POST",p,h)}
func (m *Mux) Put(p string,h http.HandlerFunc){m.handle("PUT",p,h)}
func (m *Mux) Delete(p string,h http.HandlerFunc){m.handle("DELETE",p,h)}
func (m *Mux) Route(prefix string,fn func(Router)){child:=&Mux{mux:m.mux,middleware:m.middleware,prefix:m.prefix+prefix};fn(child)}
func (m *Mux) Group(fn func(Router)){child:=&Mux{mux:m.mux,middleware:append([]func(http.Handler)http.Handler(nil),m.middleware...),prefix:m.prefix};fn(child)}
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request){ var h http.Handler=m.mux; for i:=len(m.middleware)-1;i>=0;i--{ h=m.middleware[i](h) }; h.ServeHTTP(w,r) }
type paramKey string
func URLParam(r *http.Request,name string)string{v,_:=r.Context().Value(paramKey(name)).(string);return v}
`)
	writeTestFile(t, filepath.Join(chiDir, "middleware", "middleware.go"), `package middleware
import("context";"net/http";"time")
func RequestID(next http.Handler) http.Handler{return next}
func RealIP(next http.Handler) http.Handler{return next}
func Recoverer(next http.Handler) http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){defer func(){if recover()!=nil{http.Error(w,"internal server error",500)}}();next.ServeHTTP(w,r)})}
func GetReqID(context.Context)string{return "test-request"}
func Timeout(d time.Duration) func(http.Handler) http.Handler{return func(next http.Handler) http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ctx,cancel:=context.WithTimeout(r.Context(),d);defer cancel();next.ServeHTTP(w,r.WithContext(ctx))})}}
`)

	if postgres {
		uuidDir := filepath.Join(stubs, "google-uuid")
		writeTestFile(t, filepath.Join(uuidDir, "go.mod"), "module github.com/google/uuid\n\ngo 1.23\n")
		writeTestFile(t, filepath.Join(uuidDir, "uuid.go"), `package uuid
import("encoding/hex";"errors")
type UUID [16]byte
var Nil UUID
func New()UUID{return UUID{1}}
func NewString()string{return New().String()}
func Parse(value string)(UUID,error){var id UUID;if value==""{return Nil,errors.New("invalid uuid")};compact:=value;for _,i:=range []int{23,18,13,8}{if len(compact)>i&&compact[i]=='-'{compact=compact[:i]+compact[i+1:]}};decoded,err:=hex.DecodeString(compact);if err!=nil||len(decoded)!=16{return Nil,errors.New("invalid uuid")};copy(id[:],decoded);return id,nil}
func (u UUID) String() string{return hex.EncodeToString(u[:])}
`)
		kinDir := filepath.Join(stubs, "kin-openapi")
		writeTestFile(t, filepath.Join(kinDir, "go.mod"), "module github.com/getkin/kin-openapi\n\ngo 1.23\n")
		writeTestFile(t, filepath.Join(kinDir, "openapi3", "openapi3.go"), `package openapi3
type T struct{Servers any}
type Loader struct{}
func NewLoader()*Loader{return &Loader{}}
func(*Loader)LoadFromData([]byte)(*T,error){return &T{},nil}
`)
		runtimeDir := filepath.Join(stubs, "oapi-runtime")
		writeTestFile(t, filepath.Join(runtimeDir, "go.mod"), "module github.com/oapi-codegen/runtime\n\ngo 1.23\n")
		writeTestFile(t, filepath.Join(runtimeDir, "runtime.go"), "package runtime\n")
		middlewareDir := filepath.Join(stubs, "nethttp-middleware")
		writeTestFile(t, filepath.Join(middlewareDir, "go.mod"), "module github.com/oapi-codegen/nethttp-middleware\n\ngo 1.23\n\nrequire github.com/getkin/kin-openapi v0.0.0\n\nreplace github.com/getkin/kin-openapi => "+filepath.ToSlash(kinDir)+"\n")
		writeTestFile(t, filepath.Join(middlewareDir, "middleware.go"), `package middleware
import("context";"net/http";"github.com/getkin/kin-openapi/openapi3")
type ErrorHandlerOpts struct{StatusCode int}
type Options struct{DoNotValidateServers bool;ErrorHandlerWithOpts func(context.Context,error,http.ResponseWriter,*http.Request,ErrorHandlerOpts)}
func OapiRequestValidator(*openapi3.T)func(http.Handler)http.Handler{return func(next http.Handler)http.Handler{return next}}
func OapiRequestValidatorWithOptions(*openapi3.T,*Options)func(http.Handler)http.Handler{return func(next http.Handler)http.Handler{return next}}
`)
	}
	if isProduction {
		writeProductionStubs(t, stubs)
	}
	if isDistributed {
		writeDistributedStubs(t, stubs)
	}

	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	modLines := strings.Split(string(mod), "\n")
	for index, line := range modLines {
		if strings.HasPrefix(line, "go ") {
			modLines[index] = "go 1.23"
			break
		}
	}
	modfile := strings.Join(modLines, "\n") + fmt.Sprintf("\nreplace github.com/go-chi/chi/v5 => %s\n", filepath.ToSlash(chiDir))
	if postgres {
		pgxDir := filepath.Join(stubs, "pgx")
		writePGXStub(t, pgxDir)
		modfile += fmt.Sprintf("replace github.com/jackc/pgx/v5 => %s\n", filepath.ToSlash(pgxDir))
		modfile += fmt.Sprintf("replace github.com/google/uuid => %s\n", filepath.ToSlash(filepath.Join(stubs, "google-uuid")))
		modfile += fmt.Sprintf("replace github.com/getkin/kin-openapi => %s\n", filepath.ToSlash(filepath.Join(stubs, "kin-openapi")))
		modfile += fmt.Sprintf("replace github.com/oapi-codegen/nethttp-middleware => %s\n", filepath.ToSlash(filepath.Join(stubs, "nethttp-middleware")))
		modfile += fmt.Sprintf("replace github.com/oapi-codegen/runtime => %s\n", filepath.ToSlash(filepath.Join(stubs, "oapi-runtime")))
	}
	if isProduction {
		for module, dir := range map[string]string{
			"github.com/golang-jwt/jwt/v5":                                    "jwt",
			"github.com/prometheus/client_golang":                             "prometheus",
			"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp":   "otelhttp",
			"go.opentelemetry.io/otel":                                        "otel",
			"go.opentelemetry.io/otel/sdk":                                    "otel-sdk",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc": "otlptracegrpc",
			"golang.org/x/crypto":                                             "x-crypto",
			"golang.org/x/time":                                               "x-time",
		} {
			modfile += fmt.Sprintf("replace %s => %s\n", module, filepath.ToSlash(filepath.Join(stubs, dir)))
		}
	}
	if isDistributed {
		modfile += fmt.Sprintf("replace github.com/redis/go-redis/v9 => %s\n", filepath.ToSlash(filepath.Join(stubs, "redis")))
		modfile += fmt.Sprintf("replace github.com/twmb/franz-go => %s\n", filepath.ToSlash(filepath.Join(stubs, "franz-go")))
	}
	modPath := filepath.Join(destination, "compile.mod")
	writeTestFile(t, modPath, modfile)
	if err := os.WriteFile(filepath.Join(destination, "go.mod"), []byte(modfile), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.WriteFile(filepath.Join(destination, "go.mod"), mod, 0o644); err != nil {
			t.Errorf("restore generated go.mod: %v", err)
		}
	}()
	command := exec.Command("go", "test", "-modfile=compile.mod", "./...")
	command.Dir = destination
	command.Env = append(os.Environ(), "GONOSUMDB=*", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated project does not compile: %v\n%s", err, output)
	}
	if isDistributed {
		integrationCompile := exec.Command("go", "test", "-modfile=compile.mod", "-tags=integration", "-run=^$", "./tests/integration/...")
		integrationCompile.Dir = destination
		integrationCompile.Env = append(os.Environ(), "GONOSUMDB=*", "GOSUMDB=off", "GOTOOLCHAIN=local")
		if output, err := integrationCompile.CombinedOutput(); err != nil {
			t.Fatalf("generated distributed integration tests do not compile: %v\n%s", err, output)
		}
	}
	coverage := exec.Command("go", "test", "-modfile=compile.mod", "-covermode=atomic", "-coverprofile=coverage.out", "./...")
	coverage.Dir = destination
	coverage.Env = append(os.Environ(), "GONOSUMDB=*", "GOSUMDB=off", "GOTOOLCHAIN=local")
	if output, err := coverage.CombinedOutput(); err != nil {
		t.Fatalf("generated project coverage failed: %v\n%s", err, output)
	}
	check := exec.Command(filepath.Join(destination, "scripts", "check-coverage.sh"), "coverage.out")
	check.Dir = destination
	check.Env = append(os.Environ(), "COVERAGE_MINIMUM=80")
	if output, err := check.CombinedOutput(); err != nil {
		details := exec.Command("go", "tool", "cover", "-func=coverage.filtered.out")
		details.Dir = destination
		detailOutput, _ := details.CombinedOutput()
		t.Fatalf("generated project coverage gate failed: %v\n%s\n%s", err, output, detailOutput)
	}
}

func writePGXStub(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module github.com/jackc/pgx/v5\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(root, "pgx.go"), `package pgx
import("context";"errors";"github.com/jackc/pgx/v5/pgconn")
var ErrNoRows=errors.New("no rows")
type Row interface{Scan(...any) error}
type Rows interface{Next() bool;Scan(...any) error;Err() error;Close()}
type Tx interface{Exec(context.Context,string,...any)(pgconn.CommandTag,error);Query(context.Context,string,...any)(Rows,error);QueryRow(context.Context,string,...any) Row;Commit(context.Context) error;Rollback(context.Context) error}
`)
	writeTestFile(t, filepath.Join(root, "pgconn", "pgconn.go"), `package pgconn
type CommandTag struct{}
func(CommandTag)RowsAffected()int64{return 1}
`)
	writeTestFile(t, filepath.Join(root, "pgtype", "pgtype.go"), `package pgtype
import "time"
type Timestamptz struct{Time time.Time;Valid bool}
`)
	writeTestFile(t, filepath.Join(root, "pgxpool", "pgxpool.go"), `package pgxpool
import("context";"time";"github.com/jackc/pgx/v5";"github.com/jackc/pgx/v5/pgconn")
type Config struct{MaxConns int32;MinConns int32;MaxConnLifetime time.Duration;MaxConnIdleTime time.Duration;HealthCheckPeriod time.Duration}
func ParseConfig(string)(*Config,error){return &Config{},nil}
type Pool struct{}
func NewWithConfig(context.Context,*Config)(*Pool,error){return &Pool{},nil}
func(*Pool) Ping(context.Context)error{return nil}
func(*Pool) Close(){}
func(*Pool) Begin(context.Context)(pgx.Tx,error){return tx{},nil}
func(*Pool) Exec(context.Context,string,...any)(pgconn.CommandTag,error){return pgconn.CommandTag{},nil}
func(*Pool) Query(context.Context,string,...any)(pgx.Rows,error){return rows{},nil}
func(*Pool) QueryRow(context.Context,string,...any)pgx.Row{return row{}}
type row struct{}
func(row) Scan(...any)error{return nil}
type rows struct{}
func(rows) Next()bool{return false};func(rows) Scan(...any)error{return nil};func(rows) Err()error{return nil};func(rows) Close(){}
type tx struct{}
func(tx) Exec(context.Context,string,...any)(pgconn.CommandTag,error){return pgconn.CommandTag{},nil}
func(tx) Query(context.Context,string,...any)(pgx.Rows,error){return rows{},nil}
func(tx) QueryRow(context.Context,string,...any)pgx.Row{return row{}}
func(tx) Commit(context.Context)error{return nil};func(tx) Rollback(context.Context)error{return nil}
`)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeProductionStubs(t *testing.T, root string) {
	t.Helper()
	jwtDir := filepath.Join(root, "jwt")
	writeTestFile(t, filepath.Join(jwtDir, "go.mod"), "module github.com/golang-jwt/jwt/v5\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(jwtDir, "jwt.go"), `package jwt
import "time"
type SigningMethod interface{Alg()string};type method struct{};func(method)Alg()string{return "HS256"};var SigningMethodHS256 method
type Claims interface{};type ClaimStrings []string;type NumericDate struct{Time time.Time};func NewNumericDate(t time.Time)*NumericDate{return &NumericDate{Time:t}}
type RegisteredClaims struct{Issuer,Subject string;Audience ClaimStrings;ExpiresAt,NotBefore,IssuedAt *NumericDate;ID string}
type Token struct{Method SigningMethod;Claims Claims;Valid bool};func NewWithClaims(m SigningMethod,c Claims)*Token{return &Token{Method:m,Claims:c,Valid:true}};func(*Token)SignedString(any)(string,error){return "token",nil}
type ParserOption func();func WithValidMethods([]string)ParserOption{return func(){}};func WithExpirationRequired()ParserOption{return func(){}};func WithIssuer(string)ParserOption{return func(){}};func WithAudience(string)ParserOption{return func(){}};func WithLeeway(time.Duration)ParserOption{return func(){}}
func ParseWithClaims(_ string,c Claims,key func(*Token)(any,error),_ ...ParserOption)(*Token,error){t:=&Token{Method:SigningMethodHS256,Claims:c,Valid:true};_,err:=key(t);return t,err}
`)

	promDir := filepath.Join(root, "prometheus")
	writeTestFile(t, filepath.Join(promDir, "go.mod"), "module github.com/prometheus/client_golang\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(promDir, "prometheus", "prometheus.go"), `package prometheus
import "net/http"
type CounterOpts struct{Name,Help string};type HistogramOpts struct{Name,Help string};type GaugeOpts struct{Name,Help string};type Collector interface{}
type counter struct{};func(counter)Inc(){};type observer struct{};func(observer)Observe(float64){}
type CounterVec struct{};func NewCounterVec(CounterOpts,[]string)*CounterVec{return &CounterVec{}};func(*CounterVec)WithLabelValues(...string)counter{return counter{}}
type HistogramVec struct{};func NewHistogramVec(HistogramOpts,[]string)*HistogramVec{return &HistogramVec{}};func(*HistogramVec)WithLabelValues(...string)observer{return observer{}}
type Gauge interface{Inc();Dec()};type gauge struct{};func(gauge)Inc(){};func(gauge)Dec(){};func NewGauge(GaugeOpts)Gauge{return gauge{}}
type Registry struct{};func NewRegistry()*Registry{return &Registry{}};func(*Registry)MustRegister(...Collector){}
var _ http.Handler
`)
	writeTestFile(t, filepath.Join(promDir, "prometheus", "promhttp", "promhttp.go"), `package promhttp
import("net/http";"github.com/prometheus/client_golang/prometheus")
type HandlerOpts struct{};func HandlerFor(*prometheus.Registry,HandlerOpts)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,_ *http.Request){w.WriteHeader(200)})}
`)

	otelDir := filepath.Join(root, "otel")
	writeTestFile(t, filepath.Join(otelDir, "go.mod"), "module go.opentelemetry.io/otel\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(otelDir, "otel.go"), `package otel
func SetTracerProvider(any){};func SetTextMapPropagator(any){}
`)
	writeTestFile(t, filepath.Join(otelDir, "attribute", "attribute.go"), `package attribute
type KeyValue struct{};func String(string,string)KeyValue{return KeyValue{}}
`)
	writeTestFile(t, filepath.Join(otelDir, "propagation", "propagation.go"), `package propagation
type TraceContext struct{};type Baggage struct{};func NewCompositeTextMapPropagator(...any)any{return nil}
`)
	writeTestFile(t, filepath.Join(otelDir, "trace", "trace.go"), `package trace
import "context"
type TraceID struct{};func(TraceID)String()string{return "trace"};type SpanContext struct{};func(SpanContext)IsValid()bool{return false};func(SpanContext)TraceID()TraceID{return TraceID{}};type Span struct{};func(Span)SpanContext()SpanContext{return SpanContext{}};func SpanFromContext(context.Context)Span{return Span{}}
`)

	sdkDir := filepath.Join(root, "otel-sdk")
	writeTestFile(t, filepath.Join(sdkDir, "go.mod"), "module go.opentelemetry.io/otel/sdk\n\ngo 1.23\n\nrequire go.opentelemetry.io/otel v0.0.0\n\nreplace go.opentelemetry.io/otel => "+filepath.ToSlash(otelDir)+"\n")
	writeTestFile(t, filepath.Join(sdkDir, "resource", "resource.go"), `package resource
import("context";"go.opentelemetry.io/otel/attribute")
type Resource struct{};type Option func();func WithAttributes(...attribute.KeyValue)Option{return func(){}};func New(context.Context,...Option)(*Resource,error){return &Resource{},nil}
`)
	writeTestFile(t, filepath.Join(sdkDir, "trace", "trace.go"), `package trace
import("context";"go.opentelemetry.io/otel/sdk/resource")
type TracerProvider struct{};type Option func();func WithBatcher(any)Option{return func(){}};func WithResource(*resource.Resource)Option{return func(){}};func NewTracerProvider(...Option)*TracerProvider{return &TracerProvider{}};func(*TracerProvider)Shutdown(context.Context)error{return nil}
`)

	exporterDir := filepath.Join(root, "otlptracegrpc")
	writeTestFile(t, filepath.Join(exporterDir, "go.mod"), "module go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(exporterDir, "exporter.go"), `package otlptracegrpc
import "context"
type Option func();func WithEndpoint(string)Option{return func(){}};func WithInsecure()Option{return func(){}};type Exporter struct{};func New(context.Context,...Option)(*Exporter,error){return &Exporter{},nil}
`)

	otelHTTPDir := filepath.Join(root, "otelhttp")
	writeTestFile(t, filepath.Join(otelHTTPDir, "go.mod"), "module go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(otelHTTPDir, "otelhttp.go"), `package otelhttp
import "net/http";func NewMiddleware(string,...any)func(http.Handler)http.Handler{return func(next http.Handler)http.Handler{return next}}
`)

	cryptoDir := filepath.Join(root, "x-crypto")
	writeTestFile(t, filepath.Join(cryptoDir, "go.mod"), "module golang.org/x/crypto\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(cryptoDir, "bcrypt", "bcrypt.go"), `package bcrypt
const DefaultCost=10;func CompareHashAndPassword([]byte,[]byte)error{return nil};func GenerateFromPassword(value []byte,_ int)([]byte,error){return value,nil}
`)

	timeDir := filepath.Join(root, "x-time")
	writeTestFile(t, filepath.Join(timeDir, "go.mod"), "module golang.org/x/time\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(timeDir, "rate", "rate.go"), `package rate
type Limit float64;type Limiter struct{remaining int};func NewLimiter(_ Limit,burst int)*Limiter{return &Limiter{remaining:burst}};func(l *Limiter)Allow()bool{if l.remaining<=0{return false};l.remaining--;return true}
`)
}

func writeDistributedStubs(t *testing.T, root string) {
	t.Helper()
	redisDir := filepath.Join(root, "redis")
	writeTestFile(t, filepath.Join(redisDir, "go.mod"), "module github.com/redis/go-redis/v9\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(redisDir, "redis.go"), `package redis
import("context";"time")
type Options struct{Addr,Password string;DB int};type Client struct{};func NewClient(*Options)*Client{return &Client{}}
type StatusCmd struct{err error};func(*StatusCmd)Err()error{return nil};func(*Client)Ping(context.Context)*StatusCmd{return &StatusCmd{}};func(*Client)Close()error{return nil};func(*Client)Del(context.Context,...string)*StatusCmd{return &StatusCmd{}}
type BoolCmd struct{value bool;err error};func(c *BoolCmd)Result()(bool,error){return c.value,c.err};func(*Client)SetNX(context.Context,string,any,time.Duration)*BoolCmd{return &BoolCmd{value:true}}
`)
	franzDir := filepath.Join(root, "franz-go")
	writeTestFile(t, filepath.Join(franzDir, "go.mod"), "module github.com/twmb/franz-go\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(franzDir, "pkg", "kgo", "kgo.go"), `package kgo
import "context"
type Opt struct{};func SeedBrokers(...string)Opt{return Opt{}};func ConsumerGroup(string)Opt{return Opt{}};func ConsumeTopics(...string)Opt{return Opt{}}
type Client struct{};func NewClient(...Opt)(*Client,error){return &Client{},nil};func(*Client)Ping(context.Context)error{return nil};func(*Client)Close(){}
type Record struct{Topic string;Key,Value []byte;Partition int32;Offset int64}
type ProduceResults struct{err error};func(r ProduceResults)FirstErr()error{return r.err};func(*Client)ProduceSync(context.Context,...*Record)ProduceResults{return ProduceResults{}}
type FetchError struct{Err error};type Fetches struct{records []*Record;errors []FetchError};func(Fetches)Errors()[]FetchError{return nil};func(f Fetches)EachRecord(fn func(*Record)){for _,r:=range f.records{fn(r)}};func(*Client)PollFetches(context.Context)Fetches{return Fetches{}};func(*Client)CommitRecords(context.Context,...*Record)error{return nil}
`)
}

// Package luautil embeds Lua VM into Go
package luautil

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/Akagi201/esalert/config"
	"github.com/Akagi201/esalert/context"
	lua "github.com/Shopify/go-lua"
	log "github.com/sirupsen/logrus"
)

// LuaRunner performs some arbitrary lua code. The code can either be sourced from a
// file or from a raw string (Inline).
type LuaRunner struct {
	File   string `yaml:"lua_file"`
	Inline string `yaml:"lua_inline"`
}

// Do performs the actual lua code, returning whatever the lua code returned, or
// false if there was an error
func (l *LuaRunner) Do(c context.Context) (interface{}, bool) {
	if l.File != "" {
		return RunFile(c, l.File)
	} else if l.Inline != "" {
		return RunInline(c, l.Inline)
	}
	return nil, false
}

type cmd struct {
	ctx      context.Context
	filename string
	inline   string
	retCh    chan interface{}
}

var cmdCh = make(chan cmd)

// RunInline takes the given lua code, and runs it with the given ctx variable
// set as the lua global variable "ctx". The lua code is expected to return a
// boolean value, which is passed back as the first boolean return. The second
// boolean return will be false if there was an error and the code wasn't run
func RunInline(ctx context.Context, code string) (interface{}, bool) {
	c := cmd{
		ctx:    ctx,
		inline: code,
		retCh:  make(chan interface{}),
	}
	cmdCh <- c
	ret, ok := <-c.retCh
	return ret, ok
}

// RunFile is similar to RunInline, except it takes in a filename which has the
// lua code to run. Note that the file's contents are cached, so the file is
// only opened and read the first time it's used.
func RunFile(ctx context.Context, filename string) (interface{}, bool) {
	c := cmd{
		ctx:      ctx,
		filename: filename,
		retCh:    make(chan interface{}),
	}
	cmdCh <- c
	ret, ok := <-c.retCh
	return ret, ok
}

type runner struct {
	id int // solely used to tell lua vms apart in logs
	l  *lua.State

	// Set of files and inline functions hashes already in the global namespace
	m map[string]bool
}

func init() {
	for i := 0; i < config.Opts.LuaVMs; i++ {
		newRunner(i)
	}
}

func newRunner(i int) {
	l := lua.NewState()
	lua.OpenLibraries(l)
	r := runner{
		id: i,
		l:  l,
		m:  map[string]bool{},
	}
	go r.spin()
}

func shortInline(code string) string {
	if len(code) > 20 {
		return code[:20] + " ..."
	}
	return code
}

func (r *runner) spin() {
	kv := log.Fields{
		"runnerID": r.id,
	}
	log.WithFields(kv).Debugln("initializing lua vm")

	if config.Opts.LuaInit != "" {
		initKV := log.Fields{
			"runnerID": r.id,
			"filename": config.Opts.LuaInit,
		}
		initFnName, err := r.loadFile(config.Opts.LuaInit)
		if err != nil {
			initKV["err"] = err
			log.WithFields(initKV).Fatalln("error initializing lua vm")
		}
		r.l.Global(initFnName)
		r.l.Call(0, 0)
	}

	for c := range cmdCh {
		var fnName string
		var err error
		if c.filename != "" {
			kv["filename"] = c.filename
			fnName, err = r.loadFile(c.filename)
		} else if c.inline != "" {
			kv["inline"] = shortInline(c.inline)
			fnName, err = r.loadInline(c.inline)
		}
		if err != nil {
			kv["err"] = err
			log.WithFields(kv).Errorln("error loading lua")
			close(c.retCh)
			continue
		}

		kv["fnName"] = fnName
		log.WithFields(kv).Debugln("executing lua")

		pushArbitraryValue(r.l, c.ctx)           // push ctx onto the stack
		r.l.SetGlobal("ctx")                     // set global variable "ctx" to ctx, pops it from stack
		r.l.Global(fnName)                       // push function onto stack
		r.l.Call(0, 1)                           // call function, pops function from stack, pushes return
		c.retCh <- PullArbitraryValue(r.l, true) // send back the function return, also popping it
		// stack is now clean
	}
}

func (r *runner) loadFile(name string) (string, error) {
	key := quickSha(name)
	if r.m[key] {
		return key, nil
	}

	log.WithFields(log.Fields{
		"runnerID": r.id,
		"filename": name,
		"fnName":   key,
	}).Debugln("loading lua file")
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := r.l.Load(f, name, "bt"); err != nil {
		return "", err
	}
	r.l.SetGlobal(key)

	r.m[key] = true
	return key, nil
}

func (r *runner) loadInline(code string) (string, error) {
	key := quickSha(code)
	if r.m[key] {
		return key, nil
	}

	log.WithFields(log.Fields{
		"runnerID": r.id,
		"inline":   shortInline(code),
		"fnName":   key,
	}).Debugln("loading lua inline")
	if err := r.l.Load(bytes.NewBufferString(code), key, "bt"); err != nil {
		return "", err
	}
	r.l.SetGlobal(key)

	r.m[key] = true
	return key, nil
}

func quickSha(s string) string {
	sh := sha1.New()
	sh.Write([]byte(s))
	return hex.EncodeToString(sh.Sum(nil))
}

func PullArbitraryValue(l *lua.State, remove bool) interface{} {
	if remove {
		defer l.Remove(-1)
	}
	switch t := l.TypeOf(-1); t {
	case lua.TypeNil:
		return nil
	case lua.TypeBoolean:
		return l.ToBoolean(-1)
	case lua.TypeNumber:
		i, _ := l.ToInteger(-1)
		f, _ := l.ToNumber(-1)
		if f == float64(int(f)) {
			return i
		}
		return f
	case lua.TypeString:
		s, _ := l.ToString(-1)
		return s
	case lua.TypeTable:
		m := map[string]interface{}{}
		arrSize := 0
		l.PushNil() // Next pops a value off the stack, so we add a dummy
		for l.Next(-2) {
			val := PullArbitraryValue(l, true)
			key := PullArbitraryValue(l, false)

			if keyi, ok := key.(int); ok {
				if arrSize >= 0 && arrSize < keyi {
					arrSize = keyi
				}
				key = strconv.Itoa(keyi)
			} else {
				arrSize = -1
			}

			m[key.(string)] = val
		}

		if arrSize >= 0 {
			ms := make([]interface{}, arrSize)
			for i := 0; i < arrSize; i++ {
				ms[i] = m[strconv.Itoa(i+1)]
			}
			return ms
		}

		return m
	default:
		panic(fmt.Sprintf("unknown lua type: %s", t))
	}
}

func pushArbitraryValue(l *lua.State, i interface{}) {
	if i == nil {
		l.PushNil()
		return
	}

	switch ii := i.(type) {
	case bool:
		l.PushBoolean(ii)
	case int:
		l.PushInteger(ii)
	case int8:
		l.PushInteger(int(ii))
	case int16:
		l.PushInteger(int(ii))
	case int32:
		l.PushInteger(int(ii))
	case int64:
		l.PushInteger(int(ii))
	case uint:
		l.PushUnsigned(ii)
	case uint8:
		l.PushUnsigned(uint(ii))
	case uint16:
		l.PushUnsigned(uint(ii))
	case uint32:
		l.PushUnsigned(uint(ii))
	case uint64:
		l.PushUnsigned(uint(ii))
	case float64:
		l.PushNumber(ii)
	case float32:
		l.PushNumber(float64(ii))
	case string:
		l.PushString(ii)
	case []byte:
		l.PushString(string(ii))
	default:
		v := reflect.ValueOf(i)
		switch v.Kind() {
		case reflect.Ptr:
			pushArbitraryValue(l, v.Elem().Interface())

		case reflect.Struct:
			PushTableFromStruct(l, v)

		case reflect.Map:
			PushTableFromMap(l, v)

		case reflect.Slice:
			PushTableFromSlice(l, v)

		default:
			panic(fmt.Sprintf("unknown type being pushed onto lua stack: %T %+v", i, i))
		}
	}
}

func PushTableFromStruct(l *lua.State, v reflect.Value) {
	l.NewTable()
	pushTableFromStructInner(l, v)
}

func pushTableFromStructInner(l *lua.State, v reflect.Value) {
	t := v.Type()
	for j := 0; j < v.NumField(); j++ {
		var inline bool
		name := t.Field(j).Name
		if tag := t.Field(j).Tag.Get("luautil"); tag != "" {
			tagParts := strings.Split(tag, ",")
			if tagParts[0] == "-" {
				continue
			} else if tagParts[0] != "" {
				name = tagParts[0]
			}
			if len(tagParts) > 1 && tagParts[1] == "inline" {
				inline = true
			}
		}
		if inline {
			pushTableFromStructInner(l, v.Field(j))
		} else {
			pushArbitraryValue(l, name)
			pushArbitraryValue(l, v.Field(j).Interface())
			l.SetTable(-3)
		}
	}
}

func PushTableFromMap(l *lua.State, v reflect.Value) {
	l.NewTable()
	for _, k := range v.MapKeys() {
		pushArbitraryValue(l, k.Interface())
		pushArbitraryValue(l, v.MapIndex(k).Interface())
		l.SetTable(-3)
	}
}

func PushTableFromSlice(l *lua.State, v reflect.Value) {
	l.NewTable()
	for j := 0; j < v.Len(); j++ {
		pushArbitraryValue(l, j+1) // because lua is 1-indexed
		pushArbitraryValue(l, v.Index(j).Interface())
		l.SetTable(-3)
	}
}

// Package config parses command-line/environment/config file arguments
// and make available to other packages.
package config

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

// Opts configs
var Opts struct {
	AlertFileDir      string `long:"alerts" short:"a" description:"A yaml file, or directory with yaml files, containing alert definitions"`
	ElasticSearchAddr string `long:"elasticsearch-addr" default:"127.0.0.1:9200" description:"Address to find an elasticsearch instance on"`
	LuaInit           string `long:"lua-init" description:"If set the given lua script file will be executed at the initialization of every lua vm"`
	LuaVMs            int    `long:"lua-vms" default:"1" description:"How many lua vms should be used. Each vm is completely independent of the other, and requests are executed on whatever vm is available at that moment. Allows lua scripts to not all be blocked on the same os thread"`
	SlackKey          string `long:"slack-key" description:"Slack API key, required if using any Slack actions"`
	ForceRun          string `long:"force-run" description:"If set with the name of an alert, will immediately run that alert and exit. Useful for testing changes to alert definitions"`
	LogLevel          string `long:"log-level" default:"info" description:"Adjust the log level. Valid options are: error, warn, info, debug"`
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func init() {
	parser := flags.NewParser(&Opts, flags.HelpFlag|flags.PassDoubleDash|flags.IgnoreUnknown)

	_, err := parser.Parse()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(-1)
	}
}

func init() {
	if level, err := log.ParseLevel(strings.ToLower(Opts.LogLevel)); err != nil {
		log.SetLevel(level)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
}

// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	// "github.com/davecgh/go-spew/spew"
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultConfigFilename   = "lndtool.conf"
	defaultDBFilename       = "lndtool.db"
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "admin.macaroon"
	defaultRPCPort          = "10009"
	defaultRPCHost          = "localhost"

	defaultStatsWindow = (time.Hour * 24 * 30)

	defaultFinalCLTVDelta = uint32(144)
	defaultFeeLimitRate   = float64(0.0005)

	defaultMinImbalance   = int64(1000)
	defaultTransferAmount = int64(10000)
	defaultRetryInhibit   = time.Hour
)

var (
	defaultLndDir       = btcutil.AppDataDir("lnd", false)
	defaultLndToolDir   = btcutil.AppDataDir("lndtool", false)
	defaultConfigFile   = filepath.Join(defaultLndToolDir, defaultConfigFilename)
	defaultDBFile       = filepath.Join(defaultLndToolDir, defaultDBFilename)
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultMacaroonPath = filepath.Join(
		defaultLndDir, "data", "chain", "bitcoin", "mainnet", defaultMacaroonFilename,
	)
	defaultRPCServer = defaultRPCHost + ":" + defaultRPCPort
)

type channelsConfig struct {
	StatsWindow time.Duration `long:"statswindow" description:"Time window for channel statistics"`
}

type rebalanceConfig struct {
	FinalCLTVDelta uint32  `long:"finalcltvdelta" description:"Final CLTV delta"`
	FeeLimitRate   float64 `long:"feelimitrate" description:"Limit fees to this rate"`
}

type recommendConfig struct {
	SrcChanTarget     []uint64      `long:"srcchantarget" description:"Adds channel to source target list (default: all)"`
	DstChanTarget     []uint64      `long:"dstchantarget" description:"Adds channel to destination target list (default: all)"`
	PeerNodeBlacklist []string      `long:"peernodeblacklist" description:"Adds node to peers to skip"`
	MinImbalance      int64         `long:"minimbalance" description:"Minimum imbalance to consider rebalancing"`
	TransferAmount    int64         `long:"transferamount" description:"Size of rebalance transfers"`
	RetryInhibit      time.Duration `long:"retryinhibit" description:"Inhibit retrying failed loops for this long"`
}

type config struct {
	LndDir     string `long:"lnddir" description:"The base directory that contains lnd's data, logs, configuration file, etc."`
	LndToolDir string `long:"lndtooldir" description:"The base directory that contains lndtool's data, logs, configuration file, etc."`
	ConfigFile string `long:"C" long:"configfile" description:"Path to configuration file"`
	DBFile     string `long:"dbfile" description:"Path to database file"`

	TLSCertPath string `long:"tlscertpath" description:"Path to read the TLS certificate for lnd's RPC and REST services"`

	MacaroonPath string `long:"macaroonpath" description:"path to macaroon file"`
	RPCServer    string `long:"rpcserver" description:"host:port of ln daemon"`

	Channels  *channelsConfig  `group:"Channels" namespace:"channels"`
	Rebalance *rebalanceConfig `group:"Rebalance" namespace:"rebalance"`
	Recommend *recommendConfig `group:"Recommend" namespace:"recommend"`
}

var defaultCfg = config{
	LndDir:       defaultLndDir,
	LndToolDir:   defaultLndToolDir,
	ConfigFile:   defaultConfigFile,
	DBFile:       defaultDBFile,
	TLSCertPath:  defaultTLSCertPath,
	MacaroonPath: defaultMacaroonPath,
	RPCServer:    defaultRPCServer,
	Channels: &channelsConfig{
		StatsWindow: defaultStatsWindow,
	},
	Rebalance: &rebalanceConfig{
		FinalCLTVDelta: defaultFinalCLTVDelta,
		FeeLimitRate:   defaultFeeLimitRate,
	},
	Recommend: &recommendConfig{
		SrcChanTarget:     []uint64{},
		DstChanTarget:     []uint64{},
		PeerNodeBlacklist: []string{},
		MinImbalance:      defaultMinImbalance,
		TransferAmount:    defaultTransferAmount,
		RetryInhibit:      defaultRetryInhibit,
	},
}

func nilHandler(flags.Commander, []string) error {
	return nil
}

func loadConfig() (*config, error) {
	// Pre-parse the command line options to pick up an alternative
	// config file.
	preCfg := defaultCfg
	preParser := flags.NewParser(&preCfg, flags.Default)
	addCommands(preParser)
	preParser.CommandHandler = nilHandler // disable execution on this pass
	if _, err := preParser.Parse(); err != nil {
		return nil, err
	}

	// If the config file path has not been modified by the user, then we'll
	// use the default config file path. However, if the user has modified
	// their lnddir, then we should assume they intend to use the config
	// file within it.
	configFileDir := cleanAndExpandPath(preCfg.LndToolDir)
	configFilePath := cleanAndExpandPath(preCfg.ConfigFile)
	if configFileDir != defaultLndDir {
		if configFilePath == defaultConfigFile {
			configFilePath = filepath.Join(
				configFileDir, defaultConfigFilename,
			)
		}
	}

	// Next, load any additional configuration options from the file.
	var configFileError error
	postCfg := preCfg
	if err := flags.IniParse(configFilePath, &postCfg); err != nil {
		// If it's a parsing related error, then we'll return
		// immediately, otherwise we can proceed as possibly the config
		// file doesn't exist which is OK.
		if _, ok := err.(*flags.IniError); ok {
			return nil, err
		}

		configFileError = err
	}

	// Finally, parse the remaining command line options again to ensure
	// they take precedence.
	parser := flags.NewParser(&postCfg, flags.Default)
	addCommands(parser)
	_, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	// As soon as we're done parsing configuration options, ensure all paths
	// to directories and files are cleaned and expanded before attempting
	// to use them later on.
	postCfg.TLSCertPath = cleanAndExpandPath(postCfg.TLSCertPath)
	postCfg.MacaroonPath = cleanAndExpandPath(postCfg.MacaroonPath)

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		// ltndLog.Warnf("%v", configFileError)
		fmt.Printf("%v\n", configFileError)
	}

	return &postCfg, nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

type LNDToolCommand interface {
	RunCommand() error
}

var command LNDToolCommand = nil
var arguments []string = nil

func addCommands(parser *flags.Parser) {
	parser.AddCommand("dumpconfig",
		"Dumps the configuration to stdout",
		"The dumpconfig command prints the config to stdout",
		&dumpConfigCmd)
	parser.AddCommand("channels",
		"Lists channels in tabular form",
		"Lists channels in tabular form",
		&listChannelsCmd)
	parser.AddCommand("farside",
		"Finds nodes on the far side of the connected set",
		"Finds nodes on the far side of the connected set",
		&farSideCmd)
	parser.AddCommand("rebalance",
		"Balance a pair of channels with a loop transaction",
		"Balance a pair of channels with a loop transaction",
		&rebalanceCmd)
	parser.AddCommand("recommend",
		"Recommend a pair of channels to rebalance",
		"Recommend a pair of channels to rebalance",
		&recommendCmd)
	parser.AddCommand("autobalance",
		"Loop balancing channels",
		"Loop balancing channels",
		&autoBalanceCmd)
}

type DumpConfigCmd struct {
}

var dumpConfigCmd DumpConfigCmd

func (cmd *DumpConfigCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *DumpConfigCmd) RunCommand() error {
	spew.Dump(gCfg)
	return nil
}

type ListChannelsCmd struct {
}

var listChannelsCmd ListChannelsCmd

func (cmd *ListChannelsCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *ListChannelsCmd) RunCommand() error {
	listChannels()
	return nil
}

type FarSideCmd struct {
}

var farSideCmd FarSideCmd

func (cmd *FarSideCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *FarSideCmd) RunCommand() error {
	farSide()
	return nil
}

type RebalanceCmd struct {
	Amount      int64  `short:"a" long:"amount" description:"Amount to transfer" required:"true"`
	Source      uint64 `short:"s" long:"source" description:"Source channel" required:"true"`
	Destination uint64 `short:"d" long:"destination" description:"Destination channel" required:"true"`
}

var rebalanceCmd RebalanceCmd

func (cmd *RebalanceCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *RebalanceCmd) RunCommand() error {
	doRebalance(cmd.Amount, cmd.Source, cmd.Destination)
	return nil
}

type RecommendCmd struct {
	DoIt bool `short:"d" long:"doit" description:"Execute the recommended rebalance command"`
}

var recommendCmd RecommendCmd

func (cmd *RecommendCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *RecommendCmd) RunCommand() error {
	recommend(cmd.DoIt)
	return nil
}

type AutoBalanceCmd struct {
}

var autoBalanceCmd AutoBalanceCmd

func (cmd *AutoBalanceCmd) Execute(args []string) error {
	command = cmd
	arguments = args
	return nil
}

func (cmd *AutoBalanceCmd) RunCommand() error {
	for {
		if !recommend(true) {
			break
		}
	}
	return nil
}

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
	flags "github.com/jessevdk/go-flags"
)

const (
	defaultConfigFilename   = "lndtool.conf"
	defaultTLSCertFilename  = "tls.cert"
	defaultMacaroonFilename = "admin.macaroon"
	defaultRPCPort          = "10009"
	defaultRPCHost          = "localhost"

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
	defaultTLSCertPath  = filepath.Join(defaultLndDir, defaultTLSCertFilename)
	defaultMacaroonPath = filepath.Join(
		defaultLndDir, "data", "chain", "bitcoin", "mainnet", defaultMacaroonFilename,
	)
	defaultRPCServer = defaultRPCHost + ":" + defaultRPCPort
)

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

	TLSCertPath string `long:"tlscertpath" description:"Path to read the TLS certificate for lnd's RPC and REST services"`

	MacaroonPath string `long:"macaroonpath" description:"path to macaroon file"`
	RPCServer    string `long:"rpcserver" description:"host:port of ln daemon"`

	Rebalance *rebalanceConfig `group:"Rebalance" namespace:"rebalance"`
	Recommend *recommendConfig `group:"Recommend" namespace:"recommend"`
}

func loadConfig() (*config, []string, error) {
	defaultCfg := config{
		LndDir:       defaultLndDir,
		LndToolDir:   defaultLndToolDir,
		ConfigFile:   defaultConfigFile,
		TLSCertPath:  defaultTLSCertPath,
		MacaroonPath: defaultMacaroonPath,
		RPCServer:    defaultRPCServer,
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

	// Pre-parse the command line options to pick up an alternative
	// config file.
	preCfg := defaultCfg
	if _, err := flags.Parse(&preCfg); err != nil {
		return nil, nil, err
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
	cfg := preCfg
	if err := flags.IniParse(configFilePath, &cfg); err != nil {
		// If it's a parsing related error, then we'll return
		// immediately, otherwise we can proceed as possibly the config
		// file doesn't exist which is OK.
		if _, ok := err.(*flags.IniError); ok {
			return nil, nil, err
		}

		configFileError = err
	}

	// Finally, parse the remaining command line options again to ensure
	// they take precedence.
	args, err := flags.Parse(&cfg)
	if err != nil {
		return nil, nil, err
	}

	// As soon as we're done parsing configuration options, ensure all paths
	// to directories and files are cleaned and expanded before attempting
	// to use them later on.
	cfg.TLSCertPath = cleanAndExpandPath(cfg.TLSCertPath)
	cfg.MacaroonPath = cleanAndExpandPath(cfg.MacaroonPath)

	// Warn about missing config file only after all other configuration is
	// done.  This prevents the warning on help messages and invalid
	// options.  Note this should go directly before the return.
	if configFileError != nil {
		// ltndLog.Warnf("%v", configFileError)
		fmt.Printf("%v\n", configFileError)
	}

	return &cfg, args, nil
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

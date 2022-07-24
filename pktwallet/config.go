// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	flags "github.com/jessevdk/go-flags"

	"github.com/pkt-cash/pktd/blockchain"
	"github.com/pkt-cash/pktd/btcutil"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/chaincfg/globalcfg"
	"github.com/pkt-cash/pktd/neutrino"
	"github.com/pkt-cash/pktd/pktconfig"
	"github.com/pkt-cash/pktd/pktconfig/version"
	"github.com/pkt-cash/pktd/pktlog/log"
	"github.com/pkt-cash/pktd/pktwallet/internal/cfgutil"
	"github.com/pkt-cash/pktd/pktwallet/internal/legacy/keystore"
	"github.com/pkt-cash/pktd/pktwallet/netparams"
	"github.com/pkt-cash/pktd/pktwallet/wallet"
)

const (
	defaultCAFilename       = "pktd.cert"
	defaultConfigFilename   = "pktwallet.conf"
	defaultLogLevel         = "info"
	defaultLogDirname       = "logs"
	defaultRPCMaxClients    = 10
	defaultRPCMaxWebsockets = 25
)

var (
	defaultAppDataDir = btcutil.AppDataDir("pktwallet", false)
	defaultConfigFile = filepath.Join(defaultAppDataDir, defaultConfigFilename)
	defaultLogDir     = filepath.Join(defaultAppDataDir, defaultLogDirname)
)

type config struct {
	// General application behavior
	ConfigFile    *cfgutil.ExplicitString `short:"C" long:"configfile" description:"Path to configuration file"`
	ShowVersion   bool                    `short:"V" long:"version" description:"Display version information and exit"`
	Create        bool                    `long:"create" description:"Create the wallet if it does not exist"`
	CreateTemp    bool                    `long:"createtemp" description:"Create a temporary simulation wallet (pass=password) in the data directory indicated; must call with --datadir"`
	AppDataDir    *cfgutil.ExplicitString `short:"A" long:"appdata" description:"Application data directory for wallet config, databases and logs"`
	Wallet        string                  `short:"w" long:"wallet" description:"Wallet file name or path, if a simple word such as 'personal' then pktwallet will look for wallet_personal.db, if prefixed with a / then pktwallet will consider it an absolute path."`
	TestNet3      bool                    `long:"testnet" description:"Use the test Bitcoin network (version 3) (default mainnet)"`
	PktTestNet    bool                    `long:"pkttest" description:"Use the test pkt.cash test network"`
	BtcMainNet    bool                    `long:"btc" description:"Use the test bitcoin main network"`
	PktMainNet    bool                    `long:"pkt" description:"Use the test pkt.cash main network"`
	SimNet        bool                    `long:"simnet" description:"Use the simulation test network (default mainnet)"`
	NoInitialLoad bool                    `long:"noinitialload" description:"Defer wallet creation/opening on startup and enable loading wallets over RPC"`
	DebugLevel    string                  `short:"d" long:"debuglevel" description:"Logging level {trace, debug, info, warn, error, critical}"`
	LogDir        string                  `long:"logdir" description:"Directory to log output."`
	StatsViz      string                  `long:"statsviz" description:"Enable StatsViz runtime visualization on given port -- NOTE port must be between 1024 and 65535"`
	Profile       string                  `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65535"`

	// Wallet options
	WalletPass string `long:"walletpass" default-mask:"-" description:"The public wallet password -- Only required if the wallet was created with one"`

	// SPV client options
	AddPeers     []string      `short:"a" long:"addpeer" description:"Add a peer to connect with at startup"`
	ConnectPeers []string      `long:"connect" description:"Connect only to the specified peers at startup"`
	MaxPeers     int           `long:"maxpeers" description:"Max number of inbound and outbound peers"`
	BanDuration  time.Duration `long:"banduration" description:"How long to ban misbehaving peers.  Valid time units are {s, m, h}.  Minimum 1 second"`
	BanThreshold uint32        `long:"banthreshold" description:"Maximum allowed ban score before disconnecting and banning misbehaving peers."`

	// RPC server options
	//
	// The legacy server is still enabled by default (and eventually will be
	// replaced with the experimental server) so prepare for that change by
	// renaming the struct fields (but not the configuration options).
	//
	// Usernames can also be used for the consensus RPC client, so they
	// aren't considered legacy.
	LegacyRPCListeners     []string `long:"rpclisten" description:"Listen for legacy RPC connections on this interface/port (default port: 8332, testnet: 18332, simnet: 18554)"`
	LegacyRPCMaxClients    int64    `long:"rpcmaxclients" description:"Max number of legacy RPC clients for standard connections"`
	LegacyRPCMaxWebsockets int64    `long:"rpcmaxwebsockets" description:"Max number of legacy RPC websocket connections"`
	Username               string   `short:"u" long:"rpcuser" description:"Username for legacy RPC and pktd authentication (if pktdusername is unset)"`
	Password               string   `short:"P" long:"rpcpass" default-mask:"-" description:"Password for legacy RPC and pktd authentication (if pktdpassword is unset)"`

	// These exist because btcwallet took it upon themselves to specify a username and password differently from btcd
	// in case any of these are existing in the wild, they'll be accepted.
	OldUsername string `long:"username" hidden:"true"`
	OldPassword string `long:"password" hidden:"true"`

	// Deprecated options
	DataDir *cfgutil.ExplicitString `short:"b" long:"datadir" default-mask:"-" description:"DEPRECATED -- use appdata instead"`
}

// cleanAndExpandPath expands environement variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// NOTE: The os.ExpandEnv doesn't work with Windows cmd.exe-style
	// %VARIABLE%, but they variables can still be expanded via POSIX-style
	// $VARIABLE.
	path = os.ExpandEnv(path)

	if !strings.HasPrefix(path, "~") {
		return filepath.Clean(path)
	}

	// Expand initial ~ to the current user's home directory, or ~otheruser
	// to otheruser's home directory.  On Windows, both forward and backward
	// slashes can be used.
	path = path[1:]

	var pathSeparators string
	if runtime.GOOS == "windows" {
		pathSeparators = string(os.PathSeparator) + "/"
	} else {
		pathSeparators = string(os.PathSeparator)
	}

	userName := ""
	if i := strings.IndexAny(path, pathSeparators); i != -1 {
		userName = path[:i]
		path = path[i:]
	}

	homeDir := ""
	var u *user.User
	var errr error
	if userName == "" {
		u, errr = user.Current()
	} else {
		u, errr = user.Lookup(userName)
	}
	if errr == nil {
		homeDir = u.HomeDir
	}
	// Fallback to CWD if user lookup fails or user has no home directory.
	if homeDir == "" {
		homeDir = "."
	}

	return filepath.Join(homeDir, path)
}

// loadConfig initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
//      1) Start with a default config with sane settings
//      2) Pre-parse the command line to check for an alternative config file
//      3) Load configuration file overwriting defaults with any specified options
//      4) Parse CLI options and overwrite/add any specified options
//
// The above results in pktwallet functioning properly without any config
// settings while still allowing the user to override settings with config files
// and command line options.  Command line options always take precedence.
func loadConfig() (*config, []string, er.R) {
	// Default config.
	cfg := config{
		DebugLevel:             defaultLogLevel,
		Wallet:                 "wallet.db",
		ConfigFile:             cfgutil.NewExplicitString(defaultConfigFile),
		AppDataDir:             cfgutil.NewExplicitString(defaultAppDataDir),
		LogDir:                 defaultLogDir,
		WalletPass:             wallet.InsecurePubPassphrase,
		LegacyRPCMaxClients:    defaultRPCMaxClients,
		LegacyRPCMaxWebsockets: defaultRPCMaxWebsockets,
		DataDir:                cfgutil.NewExplicitString(defaultAppDataDir),
		AddPeers:               []string{},
		ConnectPeers:           []string{},
		MaxPeers:               neutrino.MaxPeers,
		BanDuration:            neutrino.BanDuration,
		BanThreshold:           neutrino.BanThreshold,
	}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.
	preCfg := cfg
	preParser := flags.NewParser(&preCfg, flags.Default)
	_, errr := preParser.Parse()
	if errr != nil {
		if e, ok := errr.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			preParser.WriteHelp(os.Stderr)
		}
		return nil, nil, er.E(errr)
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version.Version())
		os.Exit(0)
	}

	// Load additional config from file.
	var configFileError er.R
	parser := flags.NewParser(&cfg, flags.Default)
	configFilePath := preCfg.ConfigFile.Value
	if preCfg.ConfigFile.ExplicitlySet() {
		configFilePath = cleanAndExpandPath(configFilePath)
	} else {
		appDataDir := preCfg.AppDataDir.Value
		if !preCfg.AppDataDir.ExplicitlySet() && preCfg.DataDir.ExplicitlySet() {
			appDataDir = cleanAndExpandPath(preCfg.DataDir.Value)
		}
		if appDataDir != defaultAppDataDir {
			configFilePath = filepath.Join(appDataDir, defaultConfigFilename)
		}
	}

	if errr := flags.NewIniParser(parser).ParseFile(configFilePath); errr != nil {
		if _, ok := errr.(*os.PathError); !ok {
			fmt.Fprintln(os.Stderr, errr)
			parser.WriteHelp(os.Stderr)
			return nil, nil, er.E(errr)
		}
		// log file is missing, lets create one
		fmt.Fprintln(os.Stderr, configFilePath+" does not exist, creating it from default")
		if configFileError = pktconfig.CreateDefaultConfigFile(
			configFilePath, pktconfig.PktwalletSampleConfig); configFileError != nil {
		} else {
			configFileError = er.E(flags.NewIniParser(parser).ParseFile(configFilePath))
		}
	}

	// Parse command line options again to ensure they take precedence.
	remainingArgs, errr := parser.Parse()
	if errr != nil {
		if e, ok := errr.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, nil, er.E(errr)
	}

	// Check deprecated aliases.  The new options receive priority when both
	// are changed from the default.
	if cfg.DataDir.ExplicitlySet() {
		fmt.Fprintln(os.Stderr, "datadir option has been replaced by "+
			"appdata -- please update your config")
		if !cfg.AppDataDir.ExplicitlySet() {
			cfg.AppDataDir.Value = cfg.DataDir.Value
		}
	}

	// If an alternate data directory was specified, and paths with defaults
	// relative to the data dir are unchanged, modify each path to be
	// relative to the new data dir.
	if cfg.AppDataDir.ExplicitlySet() {
		cfg.AppDataDir.Value = cleanAndExpandPath(cfg.AppDataDir.Value)
	}

	// Choose the active network params based on the selected network.
	// Multiple networks can't be selected simultaneously.
	numNets := 0
	if cfg.TestNet3 {
		activeNet = &netparams.TestNet3Params
		numNets++
	}
	if cfg.SimNet {
		activeNet = &netparams.SimNetParams
		numNets++
	}
	if cfg.PktTestNet {
		activeNet = &netparams.PktTestNetParams
		numNets++
	}
	if cfg.PktMainNet {
		activeNet = &netparams.PktMainNetParams
		numNets++
	}
	if cfg.BtcMainNet {
		activeNet = &netparams.MainNetParams
		numNets++
	}
	if numNets > 1 {
		str := "%s: The testnet and simnet params can't be used " +
			"together -- choose one"
		err := er.Errorf(str, "loadConfig")
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	// TODO(cjd): this is trash, but CompactToBig is a util function and it shouldn't
	// be in blockchain, but it is, and trying to call it from cfg is a dependency
	// loop. And duplicating the powlimit twice in the config is also trash...
	activeNet.PowLimit = blockchain.CompactToBig(activeNet.PowLimitBits)

	globalcfg.SelectConfig(activeNet.GlobalConf)

	// Append the network type to the log directory so it is "namespaced"
	// per network.
	cfg.LogDir = cleanAndExpandPath(cfg.LogDir)
	cfg.LogDir = filepath.Join(cfg.LogDir, activeNet.Params.Name)

	// Parse, validate, and set debug log level(s).
	if err := log.SetLogLevels(cfg.DebugLevel); err != nil {
		fmt.Fprintln(os.Stderr, err)
		parser.WriteHelp(os.Stderr)
		return nil, nil, err
	}

	// Exit if you try to use a simulation wallet with a standard
	// data directory.
	if !(cfg.AppDataDir.ExplicitlySet() || cfg.DataDir.ExplicitlySet()) && cfg.CreateTemp {
		fmt.Fprintln(os.Stderr, "Tried to create a temporary simulation "+
			"wallet, but failed to specify data directory!")
		os.Exit(0)
	}

	// Exit if you try to use a simulation wallet on anything other than
	// simnet or testnet3.
	if !cfg.SimNet && cfg.CreateTemp {
		fmt.Fprintln(os.Stderr, "Tried to create a temporary simulation "+
			"wallet for network other than simnet!")
		os.Exit(0)
	}

	// Ensure the wallet exists or create it when the create flag is set.
	netDir := networkDir(cfg.AppDataDir.Value, activeNet.Params)
	dbPath := wallet.WalletDbPath(netDir, cfg.Wallet)

	if cfg.CreateTemp && cfg.Create {
		err := er.Errorf("The flags --create and --createtemp can not " +
			"be specified together. Use --help for more information.")
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	dbFileExists, err := cfgutil.FileExists(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	// Ensure the data directory for the network exists.
	if err := checkCreateDir(netDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	if cfg.CreateTemp {
		tempWalletExists := false

		if dbFileExists {
			str := fmt.Sprintf("The wallet already exists. Loading this " +
				"wallet instead.")
			fmt.Fprintln(os.Stdout, str)
			tempWalletExists = true
		}

		if !tempWalletExists {
			// Perform the initial wallet creation wizard.
			if err := createSimulationWallet(&cfg); err != nil {
				fmt.Fprintln(os.Stderr, "Unable to create wallet:", err)
				return nil, nil, err
			}
		}
	} else if cfg.Create {
		// Error if the create flag is set and the wallet already
		// exists.
		if dbFileExists {
			err := er.Errorf("The wallet database file `%v` "+
				"already exists.", dbPath)
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}

		// Perform the initial wallet creation wizard.
		if err := createWallet(&cfg); err != nil {
			fmt.Fprintln(os.Stderr, "Unable to create wallet:", err)
			return nil, nil, err
		}

		// Created successfully, so exit now with success.
		os.Exit(0)
	} else if !dbFileExists && !cfg.NoInitialLoad {
		keystorePath := filepath.Join(netDir, keystore.Filename)
		keystoreExists, err := cfgutil.FileExists(keystorePath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil, nil, err
		}
		if !keystoreExists {
			err = er.Errorf("The wallet does not exist.  Run with the " +
				"--create option to initialize and create it.")
		} else {
			err = er.Errorf("The wallet is in legacy format.  Run with the " +
				"--create option to import it.")
		}
		fmt.Fprintln(os.Stderr, err)
		return nil, nil, err
	}

	neutrino.MaxPeers = cfg.MaxPeers
	neutrino.BanDuration = cfg.BanDuration
	neutrino.BanThreshold = cfg.BanThreshold

	if len(cfg.LegacyRPCListeners) == 0 {
		addrs, errr := net.LookupHost("localhost")
		if errr != nil {
			return nil, nil, er.E(errr)
		}
		cfg.LegacyRPCListeners = make([]string, 0, len(addrs))
		for _, addr := range addrs {
			addr = net.JoinHostPort(addr, activeNet.RPCServerPort)
			cfg.LegacyRPCListeners = append(cfg.LegacyRPCListeners, addr)
		}
	}

	// Add default port to all rpc listener addresses if needed and remove
	// duplicate addresses.
	cfg.LegacyRPCListeners, err = cfgutil.NormalizeAddresses(
		cfg.LegacyRPCListeners, activeNet.RPCServerPort)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Invalid network address in legacy RPC listeners: %v\n", err)
		return nil, nil, err
	}

	if cfg.Username == "" {
		cfg.Username = cfg.OldUsername
	}
	if cfg.Password == "" {
		cfg.Password = cfg.OldPassword
	}

	// Warn about missing config file after the final command line parse
	// succeeds.  This prevents the warning on help messages and invalid
	// options.
	if configFileError != nil {
		log.Warnf("%v", configFileError)
	}

	return &cfg, remainingArgs, nil
}

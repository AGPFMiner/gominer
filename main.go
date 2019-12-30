////////////////////////////////////////////////////////////////////////////
// Porgram: CommandLineCV
// Purpose: Go commandline via cobra & viper demo
// Authors: Tong Sun (c) 2015, All rights reserved
// based on https://github.com/chop-dbhi/origins-dispatch/blob/master/main.go
////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////
// Program start

package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynm/gominer/miner"
	"github.com/dynm/gominer/types"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

////////////////////////////////////////////////////////////////////////////
// Constant and data type/structure definitions

const version = "0.1.6"

// The main command describes the service and defaults to printing the
// help message.
var mainCmd = &cobra.Command{
	Use:   "gominer",
	Short: "Gominer for AGPF miners",
	Long:  `Gominer for AGPF miners`,
	Run: func(cmd *cobra.Command, args []string) {
		mine()
	},
}

// The version command prints this service.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version.",
	Long:  "The version of the dispatch service.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

var mainminer = &miner.Miner{}

// Go special automatically executed init function
func init() {
	// exec.Command("genminerconfig").Run()
	time.Sleep(1000 * time.Millisecond)

	// mainCmd.AddCommand(versionCmd)

	// flags := mainCmd.Flags()

	viper.SetDefault("device", "/dev/ttyAMA0")
	viper.SetDefault("baudrate", "115200")
	viper.SetDefault("driver", "thyroid")
	viper.SetDefault("muxnum", "1")
	viper.SetDefault("polldelay", "60")
	viper.SetDefault("noncetimeout", "1000")
	viper.SetDefault("debug", "error")
	viper.SetDefault("uartio", []int{5, 4, 3, 2})
	viper.SetDefault("jtagio", []int{26, 25, 24, 6})
	viper.SetDefault("resetio", []int{13, 12, 19, 18})
	viper.SetDefault("slot", []int{})
	viper.SetDefault("skipslots", []int{})

	// Viper supports reading from yaml, toml and/or json files. Viper can
	// search multiple paths. Paths will be searched in the order they are
	// provided. Searches stopped once Config File found.
	pflag.String("cfg", "gominer.json", "config file path")
	pflag.Bool("test", false, "test mode, build test header packet")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	fullcfgname := viper.GetString("cfg")

	log.Print("Config file: ", fullcfgname)
	cfgname := strings.TrimSuffix(fullcfgname, filepath.Ext(fullcfgname))
	if fullcfgname != "gominer.json" {
		viper.SetConfigFile(fullcfgname)

	} else {
		viper.SetConfigName(cfgname)            // name of config file (without extension)
		viper.AddConfigPath(".")                // more path to look for the config files
		viper.AddConfigPath("/opt/scripta/etc") // path to look for the config file in
	}

	err := viper.ReadInConfig()
	if err != nil {
		println("No config file found. Using built-in defaults.")
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		var pools []types.Pool
		viper.UnmarshalKey("pools", &pools)
		mainminer.Pools = pools

		mainminer.DevPath = viper.GetString("device")
		mainminer.BaudRate = viper.GetUint("baudrate")
		mainminer.Driver = viper.GetString("driver")
		mainminer.MuxNums = viper.GetInt("muxnum")
		mainminer.PollDelay = viper.GetInt64("polldelay")
		mainminer.NonceTraverseTimeout = viper.GetInt64("noncetimeout")

		mainminer.LogLevel = viper.GetString("debug")
		mainminer.Reload()
	})

}

////////////////////////////////////////////////////////////////////////////
// Main

func main() {
	mainCmd.Execute()
}

////////////////////////////////////////////////////////////////////////////
// Function definitions
func mine() {
	var pools []types.Pool
	viper.UnmarshalKey("pools", &pools)
	mainminer.Pools = pools

	mainminer.DevPath = viper.GetString("device")
	mainminer.BaudRate = viper.GetUint("baudrate")
	mainminer.Driver = viper.GetString("driver")
	mainminer.MuxNums = viper.GetInt("muxnum")
	mainminer.PollDelay = viper.GetInt64("polldelay")
	mainminer.NonceTraverseTimeout = viper.GetInt64("noncetimeout")

	mainminer.LogLevel = viper.GetString("debug")
	mainminer.MinerMain()
}

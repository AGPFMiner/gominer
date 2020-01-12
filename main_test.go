package main

import (
	"github.com/AGPFMiner/gominer/algorithms/odocrypt"
	"github.com/AGPFMiner/gominer/algorithms/skunk"
	"github.com/AGPFMiner/gominer/algorithms/veo"
	"github.com/AGPFMiner/gominer/miner"
	"github.com/AGPFMiner/gominer/types"
	"log"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/viper"
)

func TestReadConfig(t *testing.T) {
	viper.SetDefault("device", "/dev/ttyAMA0")
	viper.SetDefault("driver", "thyroid")
	viper.SetDefault("muxnum", "9")
	viper.SetDefault("polldelay", "60")
	viper.SetDefault("noncetimeout", "1000")
	viper.SetDefault("api-service", "true")
	viper.SetDefault("api-lisen", "0.0.0.0:8000")
	viper.SetDefault("debug", "error")

	viper.SetConfigName("gominer")          // name of config file (without extension)
	viper.AddConfigPath("/opt/scripta/etc") // path to look for the config file in
	viper.AddConfigPath(".")                // more path to look for the config files

	err := viper.ReadInConfig()
	if err != nil {
		println("No config file found. Using built-in defaults.")
	}

	var mainminer = &miner.Miner{}
	var pools []types.Pool
	viper.UnmarshalKey("pools", &pools)
	mainminer.Pools = pools

	mainminer.DevPath = viper.GetString("device")
	mainminer.Driver = viper.GetString("driver")
	mainminer.MuxNums = viper.GetInt("muxnum")
	mainminer.PollDelay = viper.GetInt64("polldelay")
	mainminer.NonceTraverseTimeout = viper.GetInt64("noncetimeout")

	mainminer.WebEnable = viper.GetBool("api-service")
	mainminer.WebListen = viper.GetString("api-listen")

	mainminer.LogLevel = viper.GetString("debug")
	spew.Dump(mainminer)
}

var veoPool = &types.Pool{
	URL:  "stratum+tcp://stratum.veopool.pw:8086",
	User: "BJP1y2bNVefilvrxu2YjK0PSRPcqlWWmFfECXlmryKZ9uhXzxxpfQxPBv2hZeA6vpy8MIjiAkHqD7Zo3bdFCs6o=.x86",
	Pass: "",
	Algo: "veo",
}

var skunkPool = &types.Pool{
	URL:  "stratum+tcp://skunk.hk.nicehash.com:3362",
	User: "3LbZwEmdmzhAKZqYXwbs6e4uHcH1h2wQ24.x86",
	Pass: "",
	Algo: "skunk",
}

var odoPool = &types.Pool{
	URL:  "stratum+tcp://dgb-odocrypt.f2pool.com:11115",
	User: "DEesW1UoEAUtM8mrwGHjfz1gdwPwqqRPzJ",
	Pass: "x",
	Algo: "odocrypt",
}

func TestMultiPool(t *testing.T) {
	// var pools []clients.Client
	veoCli := veo.NewClient(veoPool)
	skunkCli := skunk.NewClient(skunkPool)
	odocryptCli := odocrypt.NewClient(odoPool)
	go veoCli.Start()
	go skunkCli.Start()
	go odocryptCli.Start()
	for {
		select {
		case <-time.After(time.Second * 10):
			log.Print("VeoStats:", veoCli.PoolConnectionStates())
			log.Print("SkunkStats:", skunkCli.PoolConnectionStates())
			log.Print("OdoStats:", odocryptCli.PoolConnectionStates())
		}
	}
}

func TestMultiPoolSingleDrv(t *testing.T) {
	// var pools []clients.Client
	veoCli := veo.NewClient(veoPool)
	skunkCli := skunk.NewClient(skunkPool)
	odocryptCli := odocrypt.NewClient(odoPool)
	go veoCli.Start()
	go skunkCli.Start()
	go odocryptCli.Start()

	for {
		select {
		case <-time.After(time.Second * 10):
			log.Print("VeoStats:", veoCli.PoolConnectionStates())
			log.Print("SkunkStats:", skunkCli.PoolConnectionStates())
			log.Print("OdoStats:", odocryptCli.PoolConnectionStates())
		}
	}
}

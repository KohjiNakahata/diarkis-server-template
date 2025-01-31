// © 2019-2024 Diarkis Inc. All rights reserved.

package main

import (
	"encoding/hex"
	"fmt"
	"github.com/Diarkis/diarkis-server-template/bot/utils"
	"github.com/Diarkis/diarkis/client/go/modules/group"
	"github.com/Diarkis/diarkis/client/go/tcp"
	"github.com/Diarkis/diarkis/client/go/udp"
	"github.com/Diarkis/diarkis/util"
	"github.com/Diarkis/diarkis/uuid/v4"
	"os"
	"strconv"
	"time"
)

const UDP_STRING string = "udp"
const TCP_STRING string = "tcp"

const STATUS_BEFORE_START = 0
const STATUS_AFTER_START = 1
const STATUS_JOINING = 2
const STATUS_JOINED = 3
const STATUS_BROADCAST = 4

const BUILTIN_CMD_VER = 1
const CMD_RANDOM_JOIN_GROUP = 114

// args
var proto = "udp" // udp or tcp
var host = "127.0.0.1:7000"
var bots = 10
var packetSize = 10
var interval int64

// metrics counter
var botCounter = 0
var joinedCnt int
var broadcastCnt int

// sleepTime is in seconds
var sleepTime int64 = 1

type botData struct {
	uid   string
	rid   string
	state int
	udp   *udp.Client
	tcp   *tcp.Client
	group *group.Group
}

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Bot requires 4 parameters: {http host:port} {how many bots} {packet size} {search interval in milliseconds}")
		os.Exit(1)
		return
	}

	parseArgs()

	fmt.Printf("Starting Broadcast Bot. protocol: %v, bots num: %v, message size: %v, broadcast interval: %v\n",
		proto, bots, packetSize, interval)

	spawnBots()
	for {
		time.Sleep(time.Second * time.Duration(sleepTime))
		timestamp := util.ZuluTimeFormat(time.Now())
		fmt.Printf("{ \"Time\":\"%v\", \"Bots\":%v, \"Joined\":%v, \"Broadcast\": %v }\n",
			timestamp, botCounter, joinedCnt, broadcastCnt)
		broadcastCnt = 0
	}

	fmt.Printf("All bots have finished their works - Exiting the process - Bye!\n")
	os.Exit(0)
}

func spawnBots() {
	for i := 0; i < bots; i++ {
		id, _ := uuid.New()
		switch proto {
		case UDP_STRING:
			go spawnUDPBot(id.String, true)
		case TCP_STRING:
			go spawnTCPBot(id.String, true)
		}
	}
}

func spawnTCPBot(id string, needToWait bool) {
	if needToWait {
		waitMin := 0
		waitMax := 5000
		time.Sleep(time.Millisecond * time.Duration(int64(util.RandomInt(waitMin, waitMax))))
	}
	eResp, err := utils.Endpoint(host, id, proto)
	addr := eResp.ServerHost + ":" + fmt.Sprintf("%v", eResp.ServerPort)
	sid, _ := hex.DecodeString(eResp.Sid)
	key, _ := hex.DecodeString(eResp.EncryptionKey)
	iv, _ := hex.DecodeString(eResp.EncryptionIV)
	mkey, _ := hex.DecodeString(eResp.EncryptionMacKey)

	if err != nil {
		fmt.Printf("Auth error ID:%v - %v\n", id, err)
		return
	}
	rcvByteSize := 1400
	tcpSendInterval := int64(100)
	tcpHbInterval := int64(1000)
	tcp.LogLevel(50)
	cli := tcp.New(rcvByteSize, tcpSendInterval, tcpHbInterval)

	bot := new(botData)
	bot.uid = id
	bot.state = 0
	bot.tcp = cli

	cli.SetEncryptionKeys(sid, key, iv, mkey)
	cli.OnResponse(func(ver uint8, cmd uint16, status uint8, payload []byte) {
		handleOnResponse(bot, ver, cmd, status, payload)
	})
	cli.OnPush(func(ver uint8, cmd uint16, payload []byte) {
		handleOnPush(bot, ver, cmd, payload)
	})
	cli.OnConnect(func() {
		go startBot(bot)
	})
	cli.OnDisconnect(func() {
		fmt.Printf("Disconnected.")
		botCounter--
		joinedCnt--
		if botCounter >= bots {
			return
		}
		spawnTCPBot(bot.uid, true)
	})

	cli.Connect(addr)
	bot.group = new(group.Group)
	bot.group.SetupAsTCP(bot.tcp)
}

func spawnUDPBot(id string, needToWait bool) {
	if needToWait {
		waitMin := 0
		waitMax := 5000
		time.Sleep(time.Millisecond * time.Duration(int64(util.RandomInt(waitMin, waitMax))))
	}
	eResp, err := utils.Endpoint(host, id, proto)
	addr := eResp.ServerHost + ":" + fmt.Sprintf("%v", eResp.ServerPort)
	sid, _ := hex.DecodeString(eResp.Sid)
	key, _ := hex.DecodeString(eResp.EncryptionKey)
	iv, _ := hex.DecodeString(eResp.EncryptionIV)
	mkey, _ := hex.DecodeString(eResp.EncryptionMacKey)

	if err != nil {
		fmt.Printf("Auth error ID:%v - %v\n", id, err)
		return
	}
	rcvByteSize := 1400
	udpSendInterval := int64(100)
	udp.LogLevel(50)
	cli := udp.New(rcvByteSize, udpSendInterval)

	bot := new(botData)
	bot.uid = id
	bot.state = 0
	bot.udp = cli

	cli.SetEncryptionKeys(sid, key, iv, mkey)

	cli.OnResponse(func(ver uint8, cmd uint16, status uint8, payload []byte) {
		handleOnResponse(bot, ver, cmd, status, payload)
	})
	cli.OnPush(func(ver uint8, cmd uint16, payload []byte) {
		handleOnPush(bot, ver, cmd, payload)
	})
	cli.OnConnect(func() {
		go startBot(bot)
	})
	cli.OnDisconnect(func() {
		fmt.Printf("disconnected.\n")
		botCounter--
		if botCounter >= bots {
			return
		}
		spawnUDPBot(bot.uid, true)
	})

	cli.Connect(addr)
	bot.group = new(group.Group)
	bot.group.SetupAsUDP(bot.udp)
}

func startBot(bot *botData) {
	botCounter++

	if util.RandomInt(0, 99) < bots {
		bot.state = STATUS_AFTER_START
	}

	for {
		switch bot.state {
		case STATUS_BEFORE_START:
			bot.state = STATUS_AFTER_START
		case STATUS_AFTER_START:
			bot.state = STATUS_JOINING
			searchAndJoin(bot)
		case STATUS_JOINING:
		case STATUS_JOINED:
		case STATUS_BROADCAST:
			broadcast(bot)
		default:
			fmt.Println("This is unexpected status!!! status is %v", bot.state)
			break
		}
		time.Sleep(time.Millisecond * time.Duration(interval))
	}
}

func broadcast(bot *botData) {
	message := make([]byte, packetSize, packetSize)
	bot.group.BroadcastTo(bot.group.ID, message, false)
	broadcastCnt++
}

func searchAndJoin(bot *botData) {
	if bot.state == 0 && bot.udp == nil && bot.tcp == nil {
		fmt.Println("bot client status is invalid")
		return
	}
	groupCli := new(group.Group)
	switch proto {
	case UDP_STRING:
		groupCli.SetupAsUDP(bot.udp)
	case TCP_STRING:
		groupCli.SetupAsTCP(bot.tcp)
	}
	joinMessage := []byte("joinMessage")
	groupCli.JoinRandom(60, joinMessage, true, 200)
	bot.group = groupCli

	groupCli.OnJoin(func(success bool, groupID string) {
		joinedCnt++
		bot.state = STATUS_BROADCAST
		if success {
			bot.state = STATUS_BROADCAST
		}
	})
	groupCli.OnMemberLeave(func(message []byte) {

	})
	groupCli.OnMemberBroadcast(func(bytes []byte) {
	})
}

func handleOnResponse(bot *botData, ver uint8, cmd uint16, status uint8, payload []byte) {
}

func handleOnPush(bot *botData, ver uint8, cmd uint16, payload []byte) {
}

func parseArgs() {
	host = os.Args[1]
	botsSource, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("How many bot parameter given is invalid %v\n", err)
		os.Exit(1)
		return
	}
	bots = botsSource

	packetSource, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Printf("Packet size parameter given is invalid %v\n", err)
		os.Exit(1)
		return
	}
	packetSize = packetSource

	intervalSource, err := strconv.Atoi(os.Args[4])
	if err != nil {
		fmt.Printf("Interval of broadcast parameter given is invalid %v\n", err)
		os.Exit(1)
		return
	}
	interval = int64(intervalSource)
}

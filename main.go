package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	nats "github.com/nats-io/nats.go"
)

// CommandCenter struct
type CommandCenter struct {
	ID       string `json:"id"`
	Nick     string `json:"nick"`
	Firewall struct {
		Level float32 `json:"level"`
	} `json:"firewall"`
	Funds struct {
		Amount float32 `json:"amount"`
	} `json:"funds"`
	Scanner struct {
		Level float32 `json:"level"`
	} `json:"scanner"`
	CryptoMiner struct {
		Level float32 `json:"level"`
	} `json:"cryptoMiner"`
	Stealer struct {
		Level float32 `json:"level"`
	} `json:"stealer"`
}

// GameSettings struct
type GameSettings struct {
	minerUpdateCost    float32
	firewallUpdateCost float32
	scannerUpdateCost  float32
	stealerUpdateCost  float32
}

// UpgradeReply struct
type UpgradeReply struct {
	Allow bool    `json:"success"`
	Cost  float32 `json:"cost"`
}

// StealReply struct
type StealReply struct {
	Attacker struct {
		ID   string `json:"id"`
		Nick string `json:"nick"`
	} `json:"attacker"`
	Defender struct {
		ID   string `json:"id"`
		Nick string `json:"nick"`
	} `json:"defender"`
	Success     bool    `json:"success"`
	GainedCoins float32 `json:"gainedCoins"`
	CoolDown    bool    `json:"cooldown"`
}

// Websocket Broadcast Message
type Msg struct {
	Id   string `json:"id"`
	Data string `json:"data"`
}

var (
	nc, natsError = nats.Connect(getEnv("NATS_HOST", "localhost"), nil, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5))
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
)

func broadcastStealEvent(topic string, nc *nats.Conn, conn *websocket.Conn) {
	if _, err := nc.Subscribe(topic, func(m *nats.Msg) {
		// Initialize StealReply struct
		reply := StealReply{}
		// Load received values
		err := json.Unmarshal(m.Data, &reply)
		if err != nil {
			log.Fatalln(err)
		}
		// Publish message to websocket
		msg := Msg{
			Id:   reply.Defender.ID,
			Data: fmt.Sprintf("%s lost %f coins to %s", reply.Defender.Nick, reply.GainedCoins, reply.Attacker.Nick),
		}
		message, err := json.Marshal(msg)
		if err != nil {
			log.Fatalln(err)
		}
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		ok := conn.WriteMessage(websocket.TextMessage, message)
		if ok != nil {
			log.Println("write:", ok)
			return
		}
		// Websocket stuff
	}); err != nil {
		log.Fatal(err)
	}
}

// Publish a clients scan event on the broadcasting websocket server
func broadcastEvents(topic string, eventMessage string, nc *nats.Conn, conn *websocket.Conn) {
	if _, err := nc.Subscribe(topic, func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		// Publish message to websocket
		msg := Msg{
			Data: fmt.Sprintf("%s %s", c.Nick, eventMessage),
			Id:   c.ID,
		}
		message, err := json.Marshal(msg)
		if err != nil {
			log.Fatalln(err)
		}
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		ok := conn.WriteMessage(websocket.TextMessage, message)
		if ok != nil {
			log.Println("write:", ok)
			return
		}
		// Websocket stuff
	}); err != nil {
		log.Fatal(err)
	}
}

// Handle client miner upgrades
func minerUpdate(nc *nats.Conn, settings GameSettings, conn *websocket.Conn) {
	// Subscribe
	if _, err := nc.Subscribe("commandcenter.*.upgradeMiner", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		cost := float32(c.CryptoMiner.Level) * settings.minerUpdateCost
		if c.Funds.Amount >= cost {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: true,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			// Websocket stuff
			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their miner.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			ok := conn.WriteMessage(websocket.TextMessage, message)
			if ok != nil {
				log.Println("write:", ok)
				return
			}
			// Websocket stuff

		} else {
			// Deny commandCenter the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: false,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)
		}
	}); err != nil {
		log.Fatal(err)
	}
}

// Handle client firewall upgrades
func firewallUpdate(nc *nats.Conn, settings GameSettings, conn *websocket.Conn) {
	// Subscribe
	if _, err := nc.Subscribe("commandcenter.*.upgradeFirewall", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		cost := float32(c.Firewall.Level) * settings.firewallUpdateCost
		if c.Funds.Amount >= cost {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: true,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			// Websocket stuff
			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their firewall.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			ok := conn.WriteMessage(websocket.TextMessage, message)
			if ok != nil {
				log.Println("write:", ok)
				return
			}
			// Websocket stuff
		} else {
			// Deny commandCenter the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: false,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)
		}
	}); err != nil {
		log.Fatal(err)
	}
}

// Handle client stealer upgrades
func stealerUpdate(nc *nats.Conn, settings GameSettings, conn *websocket.Conn) {
	// Subscribe
	if _, err := nc.Subscribe("commandcenter.*.upgradeStealer", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		cost := float32(c.Stealer.Level) * settings.stealerUpdateCost
		if c.Funds.Amount >= cost {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: true,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			// Websocket stuff
			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their stealer.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			ok := conn.WriteMessage(websocket.TextMessage, message)
			if ok != nil {
				log.Println("write:", ok)
				return
			}
			// Websocket stuff
		} else {
			// Deny commandCenter the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: false,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)
		}
	}); err != nil {
		log.Fatal(err)
	}
}

// Handle client scanner upgrades
func scannerUpdate(nc *nats.Conn, settings GameSettings, conn *websocket.Conn) {
	// Subscribe
	if _, err := nc.Subscribe("commandcenter.*.upgradeScanner", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		cost := float32(c.Scanner.Level) * settings.scannerUpdateCost
		if c.Funds.Amount >= cost {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: true,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)
			// Websocket stuff
			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their scanner.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			ok := conn.WriteMessage(websocket.TextMessage, message)
			if ok != nil {
				log.Println("write:", ok)
				return
			}
			// Websocket stuff
		} else {
			// Deny commandCenter the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow: false,
				Cost:  cost,
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

		}
	}); err != nil {
		log.Fatal(err)
	}
}

// This keeps the webcocket connection alive. The server handles each client connection.
func readLoop(c *websocket.Conn) {
	for {
		if _, _, err := c.NextReader(); err != nil {
			log.Println(err)
			c.Close()
			break
		}
	}
}

// Handle setting of variables of env var is not set
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func main() {

	log.Println("Connecting to socket server.")
	u := url.URL{
		Scheme:   getEnv("SCHEME", "ws"),
		Host:     fmt.Sprintf("%s:%s", getEnv("HOST", "localhost"), getEnv("PORT", "8080")),
		Path:     "/ws",
		RawQuery: fmt.Sprintf("token=%s", getEnv("KEY", "secret")),
	}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	go readLoop(c)

	if err != nil {
		log.Fatal("dial:", err)
	}

	log.Println("Starting gamemaster.")
	wg := sync.WaitGroup{}
	wg.Add(1)

	// Set global upgrade cost
	settings := GameSettings{
		minerUpdateCost:    0.1,
		firewallUpdateCost: 0.1,
		scannerUpdateCost:  0.1,
		stealerUpdateCost:  0.1,
	}

	if natsError != nil {
		log.Fatalln(natsError)
	}
	defer nc.Close()

	// Run goroutines handling updates
	go minerUpdate(nc, settings, c)
	go firewallUpdate(nc, settings, c)
	go scannerUpdate(nc, settings, c)
	go stealerUpdate(nc, settings, c)
	go broadcastEvents("scanevent", "initiated a scan", nc, c)
	go broadcastEvents("stealevent", "is trying to steal coins", nc, c)
	go broadcastStealEvent("stealresult", nc, c)

	// TODO: Gamesettings listener (ADMIN TOOLS)
	// Run indefinitely
	wg.Wait()

}

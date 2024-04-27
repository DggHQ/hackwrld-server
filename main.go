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
	Allow  bool    `json:"success"`
	Cost   float32 `json:"cost"`
	Levels float32 `json:"levels"`
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

// Setup global variables
var (
	nc, natsError = nats.Connect(
		getEnv("NATS_HOST", "localhost"),
		nil,
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
	)
	wsmessage = make(chan []byte)
	u         = url.URL{
		Scheme:   getEnv("SCHEME", "ws"),
		Host:     fmt.Sprintf("%s:%s", getEnv("HOST", "localhost"), getEnv("PORT", "8080")),
		Path:     "/ws",
		RawQuery: fmt.Sprintf("token=%s", getEnv("KEY", "secret")),
	}
	monitor = Monitor{}
)

func (c *CommandCenter) calculateUpgrade(level float32, numUpgrades int, baseCost float32) float32 {
	costAtCurrentLevel := level * baseCost                                                           //0.1
	totalCost := float32(numUpgrades) / 2 * (2*costAtCurrentLevel + float32(numUpgrades-1)*baseCost) //0.1)
	return totalCost
}

func (c *CommandCenter) UpgradeCost(component string, numUpgrades int, settings GameSettings) float32 {
	switch component {
	case "firewall":
		return c.calculateUpgrade(c.Firewall.Level, numUpgrades, settings.firewallUpdateCost)
	case "scanner":
		return c.calculateUpgrade(c.Scanner.Level, numUpgrades, settings.scannerUpdateCost)
	case "miner":
		return c.calculateUpgrade(c.CryptoMiner.Level, numUpgrades, settings.minerUpdateCost)
	case "stealer":
		return c.calculateUpgrade(c.Stealer.Level, numUpgrades, settings.stealerUpdateCost)
	default:
		return 0.0 // or handle unknown functionality case
	}
}

func (c *CommandCenter) maxUpgrades(availableMoney float32, currentLevel int, baseCost float32) int {
	// Calculate cost at current level
	costAtCurrentLevel := float32(currentLevel) * baseCost // 0.1
	// If the cost at the current level is greater than available money, no upgrades possible
	if costAtCurrentLevel > availableMoney {
		return 0
	}
	// Initialize the maximum number of upgrades with one upgrade at the current level
	maxUpgrades := 1
	totalCost := costAtCurrentLevel
	// Calculate the cost for each additional upgrade until it exceeds available money
	for {
		// Calculate the cost for one more upgrade
		cost := float32(maxUpgrades+currentLevel) * baseCost //0.1
		// If adding one more upgrade exceeds available money, stop
		if totalCost+cost > availableMoney {
			break
		}
		// Otherwise, increment maxUpgrades and update totalCost
		maxUpgrades++
		totalCost += cost
	}
	return maxUpgrades
}

func (c *CommandCenter) MaxUpgradesByComponent(availableMoney float32, component string, currentLevel int, settings GameSettings) int {
	switch component {
	case "firewall":
		return c.maxUpgrades(availableMoney, int(c.Firewall.Level), settings.firewallUpdateCost)
	case "scanner":
		return c.maxUpgrades(availableMoney, int(c.Scanner.Level), settings.scannerUpdateCost)
	case "miner":
		return c.maxUpgrades(availableMoney, int(c.CryptoMiner.Level), settings.minerUpdateCost)
	case "stealer":
		return c.maxUpgrades(availableMoney, int(c.Stealer.Level), settings.stealerUpdateCost)
	default:
		return 0 // or handle unknown component case
	}
}

// Publish the steal event to the websocket connection
// Shows who stole from whom and how much they stole
func broadcastStealEvent(topic string, nc *nats.Conn, wsmessage chan []byte) {
	if _, err := nc.QueueSubscribe(topic, "broadcast", func(m *nats.Msg) {
		// Initialize StealReply struct
		reply := StealReply{}
		// Load received values
		err := json.Unmarshal(m.Data, &reply)
		if err != nil {
			log.Fatalln(err)
		}
		msg := Msg{
			Id:   reply.Defender.ID,
			Data: fmt.Sprintf("%s lost %f coins to %s", reply.Defender.Nick, reply.GainedCoins, reply.Attacker.Nick),
		}
		message, err := json.Marshal(msg)
		if err != nil {
			log.Fatalln(err)
		}
		// Write message to channel to be written to websocket connection
		wsmessage <- message
	}); err != nil {
		log.Fatal(err)
	}
}

// Publish a clients scan event on the broadcasting websocket server
func broadcastEvents(topic string, eventMessage string, nc *nats.Conn, wsmessage chan []byte) {
	if _, err := nc.QueueSubscribe(topic, "broadcast", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		msg := Msg{
			Data: fmt.Sprintf("%s %s", c.Nick, eventMessage),
			Id:   c.ID,
		}
		message, err := json.Marshal(msg)
		if err != nil {
			log.Fatalln(err)
		}
		// Write message to channel to be written to websocket connection
		wsmessage <- message
	}); err != nil {
		log.Fatal(err)
	}
}

func handleMinerUpgradeRequest(nc *nats.Conn, topic string, settings GameSettings, wsmessage chan []byte, maxUpgrade bool) {
	// Subscribe
	if _, err := nc.QueueSubscribe(topic, "master", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		monitor.UpgradeRequests.WithLabelValues(c.ID, c.Nick, "miner").Inc()

		var maxLevels int
		if maxUpgrade {
			maxLevels = c.MaxUpgradesByComponent(c.Funds.Amount, "miner", int(c.CryptoMiner.Level), settings)
		} else {
			maxLevels = 1
		}
		cost := c.UpgradeCost("miner", maxLevels, settings)

		if c.Funds.Amount >= cost && cost > 0 {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  true,
				Cost:   cost,
				Levels: float32(maxLevels),
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their miner.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			// Write message to channel to be written to websocket connection
			wsmessage <- message

		} else {
			// Deny commandCenter the upgrade
			// Get price for just 1 upgrade to report back to request
			cost := c.UpgradeCost("miner", 1, settings)
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  false,
				Cost:   cost,
				Levels: 0,
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

func minerUpdate(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleMinerUpgradeRequest(nc, "commandcenter.*.upgradeMiner", settings, wsmessage, false)
}
func minerUpdateMax(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleMinerUpgradeRequest(nc, "commandcenter.*.upgradeMiner.max", settings, wsmessage, true)
}

func handleFirewallUpgradeRequest(nc *nats.Conn, topic string, settings GameSettings, wsmessage chan []byte, maxUpgrade bool) {
	// Subscribe
	if _, err := nc.QueueSubscribe(topic, "master", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		monitor.UpgradeRequests.WithLabelValues(c.ID, c.Nick, "firewall").Inc()

		var maxLevels int
		if maxUpgrade {
			maxLevels = c.MaxUpgradesByComponent(c.Funds.Amount, "firewall", int(c.Firewall.Level), settings)
		} else {
			maxLevels = 1
		}
		cost := c.UpgradeCost("firewall", maxLevels, settings)

		if c.Funds.Amount >= cost && cost > 0 {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  true,
				Cost:   cost,
				Levels: float32(maxLevels),
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their firewall.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			// Write message to channel to be written to websocket connection
			wsmessage <- message

		} else {
			// Deny commandCenter the upgrade
			// Get price for just 1 upgrade to report back to request
			cost := c.UpgradeCost("firewall", 1, settings)
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  false,
				Cost:   cost,
				Levels: 0,
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
func firewallUpdate(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleFirewallUpgradeRequest(nc, "commandcenter.*.upgradeFirewall", settings, wsmessage, false)
}
func firewallUpdateMax(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleFirewallUpgradeRequest(nc, "commandcenter.*.upgradeFirewall.max", settings, wsmessage, true)
}

func handleStealerUpgradeRequest(nc *nats.Conn, topic string, settings GameSettings, wsmessage chan []byte, maxUpgrade bool) {
	// Subscribe
	if _, err := nc.QueueSubscribe(topic, "master", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		monitor.UpgradeRequests.WithLabelValues(c.ID, c.Nick, "stealer").Inc()

		var maxLevels int
		if maxUpgrade {
			maxLevels = c.MaxUpgradesByComponent(c.Funds.Amount, "stealer", int(c.Stealer.Level), settings)
		} else {
			maxLevels = 1
		}
		cost := c.UpgradeCost("stealer", maxLevels, settings)

		if c.Funds.Amount >= cost && cost > 0 {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  true,
				Cost:   cost,
				Levels: float32(maxLevels),
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their stealer.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			// Write message to channel to be written to websocket connection
			wsmessage <- message

		} else {
			// Deny commandCenter the upgrade
			// Get price for just 1 upgrade to report back to request
			cost := c.UpgradeCost("stealer", 1, settings)
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  false,
				Cost:   cost,
				Levels: 0,
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
func stealerUpdate(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleStealerUpgradeRequest(nc, "commandcenter.*.upgradeStealer", settings, wsmessage, false)
}
func stealerUpdateMax(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleStealerUpgradeRequest(nc, "commandcenter.*.upgradeStealer.max", settings, wsmessage, true)
}

func handleScannerUpgradeRequest(nc *nats.Conn, topic string, settings GameSettings, wsmessage chan []byte, maxUpgrade bool) {
	// Subscribe
	if _, err := nc.QueueSubscribe(topic, "master", func(m *nats.Msg) {
		// Initialize CommandCenter struct
		c := CommandCenter{}
		// Load received values
		err := json.Unmarshal(m.Data, &c)
		if err != nil {
			log.Fatalln(err)
		}
		monitor.UpgradeRequests.WithLabelValues(c.ID, c.Nick, "scanner").Inc()

		var maxLevels int
		if maxUpgrade {
			maxLevels = c.MaxUpgradesByComponent(c.Funds.Amount, "scanner", int(c.Stealer.Level), settings)
		} else {
			maxLevels = 1
		}
		cost := c.UpgradeCost("scanner", maxLevels, settings)

		if c.Funds.Amount >= cost && cost > 0 {
			// Allow commandCenter to purchase the upgrade
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is permitted.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  true,
				Cost:   cost,
				Levels: float32(maxLevels),
			}
			jsonReply, err := json.Marshal(reply)
			if err != nil {
				log.Fatalln(err)
			}
			m.Respond(jsonReply)

			msg := Msg{
				Data: fmt.Sprintf("%s upgraded their scanner.", c.Nick),
				Id:   c.ID,
			}
			message, err := json.Marshal(msg)
			if err != nil {
				log.Fatalln(err)
			}
			// Write message to channel to be written to websocket connection
			wsmessage <- message

		} else {
			// Deny commandCenter the upgrade
			// Get price for just 1 upgrade to report back to request
			cost := c.UpgradeCost("scanner", 1, settings)
			log.Printf("Available Funds for %s are %f. Upgrade costs %f. Upgrade is denied.", c.ID, c.Funds.Amount, cost)
			reply := UpgradeReply{
				Allow:  false,
				Cost:   cost,
				Levels: 0,
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
func scannerUpdate(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleScannerUpgradeRequest(nc, "commandcenter.*.upgradeScanner", settings, wsmessage, false)
}
func scannerUpdateMax(nc *nats.Conn, settings GameSettings, wsmessage chan []byte) {
	handleScannerUpgradeRequest(nc, "commandcenter.*.upgradeScanner.max", settings, wsmessage, true)
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

func init() {
	monitor.Init()
}

func main() {

	log.Println("Connecting to socket server.")
	log.Printf("connecting to %s", u.String())
	// Connect to websocket server
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)

	// Keep websocket connection alive
	go readLoop(c)

	if err != nil {
		log.Fatal("dial:", err)
	}

	// Launch goroutine that handles the messages that come into the channel and write them to the connection.
	go func(connection *websocket.Conn) {
		for msg := range wsmessage {
			// Dunnot if writedeadline is needed since it seems to be working fine w/o it
			//c.SetWriteDeadline(time.Now().Add(writeWait))
			ok := c.WriteMessage(websocket.TextMessage, msg)
			if ok != nil {
				log.Println("write:", ok)
			}
		}
	}(c)

	log.Println("Starting gamemaster.")
	// Set waitgroup to keep program running forever
	wg := sync.WaitGroup{}
	wg.Add(1)

	// Set global upgrade cost
	settings := GameSettings{
		minerUpdateCost:    0.1,
		firewallUpdateCost: 0.1,
		scannerUpdateCost:  0.1,
		stealerUpdateCost:  0.1,
	}

	// Log when nats cannot connect
	if natsError != nil {
		log.Fatalln(natsError)
	}
	defer nc.Close()

	// Run goroutines handling updates and broadcasts for the websocket connection
	go minerUpdate(nc, settings, wsmessage)
	go minerUpdateMax(nc, settings, wsmessage)
	go firewallUpdate(nc, settings, wsmessage)
	go firewallUpdateMax(nc, settings, wsmessage)
	go scannerUpdate(nc, settings, wsmessage)
	go scannerUpdateMax(nc, settings, wsmessage)
	go stealerUpdate(nc, settings, wsmessage)
	go stealerUpdateMax(nc, settings, wsmessage)
	go broadcastEvents("scanevent", "initiated a scan", nc, wsmessage)
	go broadcastEvents("stealevent", "is trying to steal coins", nc, wsmessage)
	go broadcastStealEvent("stealresult", nc, wsmessage)
	go monitor.Run()

	// TODO: Gamesettings listener (ADMIN TOOLS)
	// At some point this game master could be configured remotely to dynamically change cost of components
	// This might be a future feature.

	// Run indefinitely
	wg.Wait()

}

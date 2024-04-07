package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	nats "github.com/nats-io/nats.go"
)

// CommandCenter struct
type CommandCenter struct {
	ID       string `json:"id"`
	Firewall struct {
		Level int `json:"level"`
	} `json:"firewall"`
	Funds struct {
		Amount float32 `json:"amount"`
	} `json:"funds"`
	Scanner struct {
		Level int `json:"level"`
	} `json:"scanner"`
	CryptoMiner struct {
		Level int `json:"level"`
	} `json:"cryptoMiner"`
}

// GameSettings struct
type GameSettings struct {
	minerUpdateCost    float32
	firewallUpdateCost float32
	scannerUpdateCost  float32
}

// UpgradeReply struct
type UpgradeReply struct {
	Allow bool
	Cost  float32
}

var (
	nc, natsError = nats.Connect("localhost", nil, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5))
)

func minerUpdate(nc *nats.Conn, settings GameSettings, wg *sync.WaitGroup) {
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

func firewallUpdate(nc *nats.Conn, settings GameSettings, wg *sync.WaitGroup) {
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

func scannerUpdate(nc *nats.Conn, settings GameSettings, wg *sync.WaitGroup) {
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

func main() {
	wg := sync.WaitGroup{}
	wg.Add(3)

	settings := GameSettings{
		minerUpdateCost:    0.1,
		firewallUpdateCost: 0.1,
		scannerUpdateCost:  0.1,
	}

	if natsError != nil {
		log.Fatalln(natsError)
	}
	defer nc.Close()

	go minerUpdate(nc, settings, &wg)
	go firewallUpdate(nc, settings, &wg)
	go scannerUpdate(nc, settings, &wg)
	//TODO: Gamesettings listener (ADMIN TOOLS)

	wg.Wait()

}

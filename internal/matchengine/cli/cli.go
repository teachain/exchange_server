package cli

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/teachain/exchange_server/internal/matchengine/engine"
	"github.com/teachain/exchange_server/internal/matchengine/order"
)

type CLI struct {
	addr   string
	engine *engine.Engine
	ln     net.Listener
	wg     sync.WaitGroup
	stopCh chan struct{}
}

func NewCLI(addr string, e *engine.Engine) *CLI {
	return &CLI{
		addr:   addr,
		engine: e,
		stopCh: make(chan struct{}),
	}
}

func (c *CLI) Start() error {
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}
	c.ln = ln

	c.wg.Add(1)
	go c.acceptLoop()

	return nil
}

func (c *CLI) Stop() {
	close(c.stopCh)
	if c.ln != nil {
		c.ln.Close()
	}
	c.wg.Wait()
}

func (c *CLI) acceptLoop() {
	defer c.wg.Done()

	for {
		conn, err := c.ln.Accept()
		if err != nil {
			select {
			case <-c.stopCh:
				return
			default:
				continue
			}
		}

		c.wg.Add(1)
		go c.handleConn(conn)
	}
}

func (c *CLI) handleConn(conn net.Conn) {
	defer c.wg.Done()
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Minute))

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		var resp string
		switch cmd {
		case "status":
			resp = c.cmdStatus()
		case "balance":
			resp = c.cmdBalance(args)
		case "market":
			resp = c.cmdMarket(args)
		case "makeslice":
			resp = c.cmdMakeSlice()
		case "help":
			resp = "Commands: status, balance list/get/summary, market summary, makeslice\n"
		default:
			resp = fmt.Sprintf("unknown command: %s\n", cmd)
		}

		conn.Write([]byte(resp))
	}
}

func (c *CLI) cmdStatus() string {
	return fmt.Sprintf(`Market:     running
OrderBook: %d markets loaded
Time:      %d
`, len(c.engine.ListMarkets()), time.Now().Unix())
}

func (c *CLI) cmdBalance(args []string) string {
	if len(args) == 0 {
		return "usage: balance list/get/summary\n"
	}

	switch args[0] {
	case "list":
		return c.cmdBalanceList(args[1:])
	case "get":
		return c.cmdBalanceGet(args[1:])
	case "summary":
		return c.cmdBalanceSummary()
	default:
		return "usage: balance list/get/summary\n"
	}
}

func (c *CLI) cmdBalanceList(args []string) string {
	var asset string
	if len(args) > 0 {
		asset = args[0]
	}

	assetList := c.engine.ListAssets()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%-10s %-16s %-10s %s\n", "user", "asset", "type", "amount"))

	users := make(map[uint32]bool)
	for _, marketName := range c.engine.ListMarkets() {
		ob, ok := c.engine.GetOrderBook(marketName)
		if !ok {
			continue
		}
		for _, o := range ob.GetOrders() {
			users[o.UserID] = true
		}
	}

	for userID := range users {
		for _, assetName := range assetList {
			available, frozen := c.engine.GetBalance(userID, assetName)
			if available.IsZero() && frozen.IsZero() {
				continue
			}
			if asset != "" && assetName != asset {
				continue
			}
			if !available.IsZero() {
				result.WriteString(fmt.Sprintf("%-10d %-16s %-10s %s\n", userID, assetName, "available", available.String()))
			}
			if !frozen.IsZero() {
				result.WriteString(fmt.Sprintf("%-10d %-16s %-10s %s\n", userID, assetName, "freeze", frozen.String()))
			}
		}
	}

	return result.String()
}

func (c *CLI) cmdBalanceGet(args []string) string {
	if len(args) < 1 {
		return "usage: balance get user_id [asset]\n"
	}

	var userID uint32
	_, err := fmt.Sscanf(args[0], "%d", &userID)
	if err != nil {
		return fmt.Sprintf("invalid user_id: %s\n", args[0])
	}

	var asset string
	if len(args) > 1 {
		asset = args[1]
	}

	assetList := c.engine.ListAssets()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%-10s %-16s %-10s %s\n", "user", "asset", "type", "amount"))

	for _, assetName := range assetList {
		if asset != "" && assetName != asset {
			continue
		}
		available, frozen := c.engine.GetBalance(userID, assetName)
		if available.IsZero() && frozen.IsZero() {
			continue
		}
		if !available.IsZero() {
			result.WriteString(fmt.Sprintf("%-10d %-16s %-10s %s\n", userID, assetName, "available", available.String()))
		}
		if !frozen.IsZero() {
			result.WriteString(fmt.Sprintf("%-10d %-16s %-10s %s\n", userID, assetName, "freeze", frozen.String()))
		}
	}

	return result.String()
}

func (c *CLI) cmdBalanceSummary() string {
	assetList := c.engine.ListAssets()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%-16s %-30s %-10s %-30s %-10s %-30s\n", "asset", "total", "avail_count", "available", "freeze_count", "freeze"))

	for _, assetName := range assetList {
		var totalAvailable, totalFrozen decimal.Decimal
		availCount := 0
		freezeCount := 0

		users := make(map[uint32]bool)
		for _, marketName := range c.engine.ListMarkets() {
			ob, ok := c.engine.GetOrderBook(marketName)
			if !ok {
				continue
			}
			for _, o := range ob.GetOrders() {
				users[o.UserID] = true
			}
		}

		for userID := range users {
			available, frozen := c.engine.GetBalance(userID, assetName)
			if !available.IsZero() {
				totalAvailable = totalAvailable.Add(available)
				availCount++
			}
			if !frozen.IsZero() {
				totalFrozen = totalFrozen.Add(frozen)
				freezeCount++
			}
		}

		total := totalAvailable.Add(totalFrozen)
		result.WriteString(fmt.Sprintf("%-16s %-30s %-10d %-30s %-10d %-30s\n",
			assetName, total.String(), availCount, totalAvailable.String(), freezeCount, totalFrozen.String()))
	}

	return result.String()
}

func (c *CLI) cmdMarket(args []string) string {
	if len(args) == 0 {
		return "usage: market summary\n"
	}

	switch args[0] {
	case "summary":
		return c.cmdMarketSummary()
	default:
		return "usage: market summary\n"
	}
}

func (c *CLI) cmdMarketSummary() string {
	markets := c.engine.ListMarkets()

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%-10s %-10s %-20s %-10s %-20s\n", "market", "ask count", "ask amount", "bid count", "bid amount"))

	for _, marketName := range markets {
		ob, ok := c.engine.GetOrderBook(marketName)
		if !ok {
			continue
		}

		var askCount, bidCount int
		var askAmount, bidAmount decimal.Decimal

		for _, o := range ob.GetOrders() {
			if o.Side == order.SideAsk {
				askCount++
				askAmount = askAmount.Add(o.Left)
			} else {
				bidCount++
				bidAmount = bidAmount.Add(o.Left)
			}
		}

		result.WriteString(fmt.Sprintf("%-10s %-10d %-20s %-10d %-20s\n", marketName, askCount, askAmount.String(), bidCount, bidAmount.String()))
	}

	return result.String()
}

func (c *CLI) cmdMakeSlice() string {
	return "OK\n"
}

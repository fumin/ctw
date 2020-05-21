package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/fumin/ctw"
	"github.com/fumin/ctw/app/taifx/mcts"
	"github.com/pkg/errors"
)

var (
	flagConfig = flag.String("c", `{
		"Data": "txf_renko_0001.csv",
		"PriceDelta": 0.001
		"TransactionCost": 0.5,
		"Depth": 48,
		"Leverage": 3,
		}`, "configuration")
)

type Bar struct {
	Time      time.Time
	Price     float64
	Direction int
}

func parseData(config Config) ([]Bar, []Bar, error) {
	f, err := os.Open(config.Data)
	if err != nil {
		return nil, nil, errors.Wrap(err, "")
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, nil, errors.Wrap(err, "")
	}
	// Remove header.
	records = records[1:]

	train := make([]Bar, 0, 1024)
	test := make([]Bar, 0, 1024)
	for _, r := range records {
		timeStr := r[1]
		priceStr := r[3]
		directionStr := r[8]

		t, err := time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return nil, nil, errors.Wrap(err, fmt.Sprintf("%+v", r))
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return nil, nil, errors.Wrap(err, fmt.Sprintf("%+v", r))
		}
		var direction int
		switch directionStr {
		case "True":
			direction = 1
		case "False":
			direction = 0
		default:
			return nil, nil, errors.Wrap(err, fmt.Sprintf("%+v", r))
		}
		bar := Bar{}
		bar.Time = t
		bar.Price = price
		bar.Direction = direction

		if t.Before(time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)) {
			train = append(train, bar)
		} else {
			test = append(test, bar)
		}
	}

	return train, test, nil
}

type Data struct {
	Bar    []Bar
	Cursor int
}

func NewData(bar []Bar) *Data {
	d := &Data{}
	d.Bar = bar
	d.Cursor = 0
	return d
}

func (d *Data) Consume() Bar {
	bar := d.Bar[d.Cursor]
	d.Cursor++
	return bar
}

type StatItem struct {
	Time       time.Time
	Price      float64
	Action int
	Position int
	TransactionCost float64
	ProfitLoss float64
	Balance    float64
}

type Stat struct {
	TransactionCost float64
	Leverage float64
	Items    []StatItem
}

func NewStat(transactionCost, leverage float64, item StatItem) *Stat {
	s := &Stat{}
	s.TransactionCost = transactionCost
	s.Leverage = leverage
	s.Items = make([]StatItem, 0, 1024)
	s.Items = append(s.Items, item)
	return s
}

func (s *Stat) Record(action int, nextBar Bar) {
	prevItem := s.Items[len(s.Items)-1]

	item := StatItem{}
	item.Time = nextBar.Time
	item.Price = nextBar.Price
	item.Action = action
	item.Position = action * int(prevItem.Balance / prevItem.Price * s.Leverage)
	item.TransactionCost = s.TransactionCost * math.Abs(float64(item.Position - prevItem.Position))

	profitLoss := nextBar.Price - prevItem.Price
	profitLoss *= float64(item.Position)
	item.ProfitLoss = profitLoss

	item.Balance = prevItem.Balance + profitLoss - item.TransactionCost

	s.Items = append(s.Items, item)
}

func (s *Stat) Bankrupt() bool {
	item := s.Items[len(s.Items)-1]
	if item.Balance < 0 {
		return true
	}
	return false
}

type nextStep struct{}

func (a *nextStep) trade(model *ctw.CTW) int {
	prob0 := model.Prob0()
	if prob0 > 0.5 {
		return -1
	}
	return 1
}

type mctsAgent struct{
	priceDelta float64
	tcost float64
	algo *mcts.MCTS
	states []mctsState
}

func newMCTSAgent(priceDelta, tcost float64, steps int) *mctsAgent{
	agent := &mctsAgent{}
	agent.priceDelta = priceDelta
	agent.tcost = tcost
	agent.algo = mcts.NewMCTS()
	// plus 1 for the root state.
	agent.states = make([]mctsState, steps+1)
	return agent
}

type mctsState struct{
	price float64
	position int
}

type mctsEnv struct{
	priceDelta float64
	tcost float64
	reverter *ctw.CTWReverter
	states []mctsState
	stateCursor int
}

func (env *mctsEnv) NumActions() int {
	if env.stateCursor + 1 >= len(env.states) {
		return 0
	}
	// no inter trades
	if env.stateCursor > 1 {
		return 1
	}
	return 3
}

func (env *mctsEnv) Act(action int) {
	next := mctsState{}

	prob0 := env.reverter.Prob0()
	direction := 0
	if rand.Float64() > prob0 {
		direction = 1
	}

	s := env.states[env.stateCursor]
	var priceChg float64 = math.Ceil(s.price * env.priceDelta)
	if direction == 0 {
		priceChg *= -1
	}
	next.price = s.price + priceChg

	switch action {
	case 0: next.position = -1
	case 1: next.position = 0
	case 2: next.position = 1
	default: log.Fatalf("%d", action)
	}

	switch env.stateCursor {
	case 0:
	default:
		// Avoid excessive trading.
		next.position = s.position
	}

	env.reverter.Observe(direction)
	env.stateCursor += 1
	env.states[env.stateCursor] = next
}

func (env *mctsEnv) Reward() float64 {
	s := env.states[env.stateCursor]
	// Happens only for the root node.
	if env.stateCursor - 1 < 0 {
		return 0
	}
	prev := env.states[env.stateCursor-1]

	posChg := s.position - prev.position
	transactionCost := math.Abs(float64(posChg)) * env.tcost

	profitLoss := s.price - prev.price
	profitLoss *= float64(s.position)

//log.Printf("%+v %+v %f %f", prev, s, transactionCost, profitLoss)

	return profitLoss - transactionCost
}

func (agent *mctsAgent) trade(model *ctw.CTW, price float64, position int) int {
	env := &mctsEnv{}
	env.priceDelta = agent.priceDelta
	env.tcost = agent.tcost
	env.reverter = ctw.NewCTWReverter(model)
	env.states = agent.states
	env.states[0] = mctsState{price: price, position: position}
	agent.algo.NewRoot()

	prob0 := env.reverter.Prob0()
//log.Printf("prob0 %f", prob0)

	for i := 0; i < 8192; i++ {
		env.stateCursor = 0
		// Exploration should roughly be the magnitude of the value function.
		// Since our model follows brownian motion pretty closely, and the price is roughly 10000, priceDelta 0.001, steps 24,
		// the value is 10000 * 0.001 * sqrt(24) == 49.
		var exploration float64 = 100
//log.Printf("rollout")
		agent.algo.Rollout(env, exploration)

		// Reset state.
		for j := 0; j < env.stateCursor; j++ {
			env.reverter.Unobserve()
		}

		// Check reset.
		p0 := env.reverter.Prob0()
		if p0 != prob0 {
			log.Fatalf("%f %f", p0, prob0)
		}
	}

	action := agent.algo.BestAction()
	var trade int
	switch action {
	case 0: trade = -1
	case 1: trade = 0
	case 2: trade = 1
	default: log.Fatalf("%d", action)
	}

	agent.algo.ReleaseMem()

	return trade
}

func run(config Config) error {
	trainBar, testBar, err := parseData(config)
	if err != nil {
		return errors.Wrap(err, "")
	}

	log.Printf("train %+v", trainBar[:3])
	log.Printf("test %+v", testBar[:3])

	trainData := NewData(trainBar)
	testData := NewData(testBar)

	context := make([]int, 0, config.Depth)
	for i := 0; i < config.Depth; i++ {
		context = append(context, trainData.Consume().Direction)
	}
	model := ctw.NewCTW(context)

	// Train.
	for {
		if trainData.Cursor >= len(trainData.Bar) {
			break
		}
		bar := trainData.Consume()
		model.Observe(bar.Direction)
	}

	// Test.
	curBar := trainData.Bar[len(trainData.Bar)-1]
	item0 := StatItem{}
	item0.Time = curBar.Time
	item0.Price = curBar.Price
	item0.Balance = 20000
	testStat := NewStat(config.TransactionCost, config.Leverage, item0)
	// agent := nextStep{}
	agent := newMCTSAgent(config.PriceDelta, config.TransactionCost, 24)
	fmt.Printf("time,price,action,position,transactionCost,profitLoss,balance\n")
	step := 0
	for {
		var action int
		if step % 24 == 0 {
			curItem := testStat.Items[len(testStat.Items)-1]
			action = agent.trade(model, curItem.Price, curItem.Position)
		} else {
			curItem := testStat.Items[len(testStat.Items)-1]
			action = curItem.Action
		}
		step++

		if testData.Cursor >= len(testData.Bar) {
			break
		}
		nextBar := testData.Consume()

		testStat.Record(action, nextBar)
		if testStat.Bankrupt() {
			break
		}

		model.Observe(nextBar.Direction)

		s := testStat.Items[len(testStat.Items)-1]
		fmt.Printf("%s,%.0f,%d,%d,%.2f,%.0f,%.2f\n", s.Time.Format("2006-01-02 15:04:05"), s.Price, s.Action, s.Position, s.TransactionCost, s.ProfitLoss, s.Balance)
	}

	// fmt.Printf("time,price,action,position,transactionCost,profitLoss,balance\n")
	// for _, s := range testStat.Items {
	// 	fmt.Printf("%s,%.0f,%d,%d,%.2f,%.0f,%.2f\n", s.Time.Format("2006-01-02 15:04:05"), s.Price, s.Action, s.Position, s.TransactionCost, s.ProfitLoss, s.Balance)
	// }

	return nil
}

type Config struct {
	Data  string
	PriceDelta float64
	TransactionCost float64
	Depth int
	Leverage float64
}

func parseConfig() (Config, error) {
	config := Config{}
	if err := json.Unmarshal([]byte(*flagConfig), &config); err != nil {
		return Config{}, errors.Wrap(err, "")
	}
	configB, err := json.Marshal(config)
	if err != nil {
		return Config{}, errors.Wrap(err, "")
	}
	log.Printf("config: %s", configB)
	return config, nil
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)

	config, err := parseConfig()
	if err != nil {
		log.Fatalf("%+v", err)
	}
	if err := run(config); err != nil {
		log.Fatalf("%+v", err)
	}
}

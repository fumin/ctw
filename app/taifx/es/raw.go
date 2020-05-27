package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/fumin/ctw"
	"github.com/pkg/errors"
)

var (
	flagConfig = flag.String("c", `{
		"Seed": 0,
                "Data": "/Users/mac/Desktop/es_1m.csv",
		"Threashold": 0.002,
                "TransactionCost": 0.05,
                "Depth": 48,
		"Leverage": 1,
		"Balance": 10000
                }`, "configuration")
)

type Data struct {
	f *os.File
	r *csv.Reader
}

func NewData(config Config) (*Data, error) {
	data := &Data{}

	var err error
	data.f, err = os.Open(config.Data)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}
	data.r = csv.NewReader(data.f)
	if err != nil {
		return nil, errors.Wrap(err, "")
	}
	// Remove header.
	if _, err := data.r.Read(); err != nil {
		return nil, errors.Wrap(err, "")
	}

	return data, nil
}

func (data *Data) Close() error {
	if err := data.f.Close(); err != nil {
		return errors.Wrap(err, "")
	}
	return nil
}

type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

func (data *Data) Read() (Candle, error) {
	rec, err := data.r.Read()
	if err != nil {
		return Candle{}, errors.Wrap(err, "")
	}

	dtStr := rec[0]
	timeStr := rec[1]
	t, err := time.Parse("01/02/2006 15:04", dtStr+" "+timeStr)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}
	c := Candle{}
	c.Time = t

	c.Open, err = strconv.ParseFloat(rec[2], 64)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}
	c.High, err = strconv.ParseFloat(rec[3], 64)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}
	c.Low, err = strconv.ParseFloat(rec[4], 64)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}
	c.Close, err = strconv.ParseFloat(rec[5], 64)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}
	c.Volume, err = strconv.ParseInt(rec[6], 10, 64)
	if err != nil {
		return Candle{}, errors.Wrap(err, fmt.Sprintf("%+v", rec))
	}

	return c, nil
}

type Renko struct {
	Time      time.Time
	Price     float64
	Direction int
}

type Agent interface {
	SetModel(*ctw.CTW)
	Observe(Renko)
	Act(float64, float64, int) int
}

type RenkoWrapper struct {
	Threashold float64
	Depth      int
	curCandle  *Candle
	context    []int
	Agent      Agent
}

func NewRenkoWrapper(config Config) *RenkoWrapper {
	wrapper := &RenkoWrapper{}
	wrapper.Threashold = config.Threashold
	wrapper.Depth = config.Depth
	return wrapper
}

func (wrapper *RenkoWrapper) Observe(candle Candle) *Renko {
	if wrapper.curCandle == nil {
		wrapper.curCandle = &candle
		return nil
	}

	direction := -1
	if candle.Close/wrapper.curCandle.Close > 1+wrapper.Threashold {
		wrapper.curCandle = &candle
		direction = 1
	}
	if candle.Close/wrapper.curCandle.Close < 1-wrapper.Threashold {
		wrapper.curCandle = &candle
		direction = 0
	}
	if direction == -1 {
		return nil
	}

	if len(wrapper.context) < wrapper.Depth {
		wrapper.context = append(wrapper.context, direction)
		if len(wrapper.context) < wrapper.Depth {
			return nil
		}
		model := ctw.NewCTW(wrapper.context)
		wrapper.Agent.SetModel(model)
		return nil
	}

	renko := Renko{Time: candle.Time, Price: candle.Close, Direction: direction}
	wrapper.Agent.Observe(renko)
	return &renko
}

func (wrapper *RenkoWrapper) Act(candle Candle, balance float64, position int) (int, *Renko) {
	renko := wrapper.Observe(candle)
	if renko == nil {
		return position, nil
	}
	return wrapper.Agent.Act(renko.Price, balance, position), renko
}

type Entry struct {
	Time            time.Time
	Price           float64
	Position        int
	TransactionCost float64
	ProfitLoss      float64
	Balance         float64
}

type Tester struct {
	TransactionCost float64
	History         []Entry
	MaxHistory      int

	Trials   float64
	Corrects float64
}

func NewTester(config Config, prevCandle Candle) *Tester {
	tester := &Tester{}
	tester.TransactionCost = config.TransactionCost
	tester.MaxHistory = 128

	entry := Entry{}
	entry.Time = prevCandle.Time
	entry.Price = prevCandle.Close
	entry.Balance = config.Balance
	tester.History = append(tester.History, entry)

	return tester
}

func (tester *Tester) Record(position int, candle Candle) {
	prev := tester.History[len(tester.History)-1]

	posChg := math.Abs(float64(position - prev.Position))
	tcost := posChg * tester.TransactionCost

	profitLoss := (candle.Close - prev.Price) * float64(prev.Position)

	entry := Entry{}
	entry.Time = candle.Time
	entry.Price = candle.Close
	entry.Position = position
	entry.TransactionCost = tcost
	entry.ProfitLoss = profitLoss
	entry.Balance = prev.Balance - tcost + profitLoss
	tester.History = append(tester.History, entry)

	if len(tester.History) > tester.MaxHistory {
		tester.trim()
	}
}

func (tester *Tester) PrintCSV() {
	h := tester.History[len(tester.History)-1]
	tStr := h.Time.Format("2006-01-02 15:04")
	fmt.Printf("%s,%.2f,%d,%.2f,%.2f,%.2f\n", tStr, h.Price, h.Position, h.TransactionCost, h.ProfitLoss, h.Balance)
}

func (tester *Tester) trim() {
	start := len(tester.History) / 2
	for i := start; i < len(tester.History); i++ {
		tester.History[i-start] = tester.History[i]
	}
	numLeft := len(tester.History) - start
	tester.History = tester.History[:numLeft]
}

type NextStep struct {
	Leverage float64
	Model    *ctw.CTW
}

func (agent *NextStep) SetModel(model *ctw.CTW) {
	agent.Model = model
}

func (agent *NextStep) Observe(rk Renko) {
	agent.Model.Observe(rk.Direction)
}

func (agent *NextStep) Act(price, balance float64, prevPos int) int {
	pos := int(balance / price * agent.Leverage)
	prob0 := agent.Model.Prob0()
	if prob0 > 0.5 {
		pos = -pos
	}

	return pos
}

type RolloutAgent struct {
	Threashold      float64
	TransactionCost float64
	Leverage        float64
	Depth           int
	NumSimulations  int
	model           *ctw.CTW
	reverter        *ctw.CTWReverter

	tick int
}

func (agent *RolloutAgent) SetModel(model *ctw.CTW) {
	agent.model = model
	agent.reverter = ctw.NewCTWReverter(model)
}

func (agent *RolloutAgent) Observe(rk Renko) {
	agent.model.Observe(rk.Direction)
}

func (agent *RolloutAgent) Act(price, balance float64, prevPos int) int {
	agent.tick++
	if agent.tick < agent.Depth {
		return prevPos
	}
	agent.tick = 0

	var nextPrice float64
	prob0 := agent.model.Prob0()
	for i := 0; i < agent.NumSimulations; i++ {
		if agent.model.Prob0() != prob0 {
			log.Fatalf("%f %f", agent.model.Prob0(), prob0)
		}
		nextPrice += agent.rollout(price)
	}
	nextPrice /= float64(agent.NumSimulations)

	pos := int(balance / price * agent.Leverage)
	longPL := agent.profitLoss(price, nextPrice, prevPos, pos)
	shortPL := agent.profitLoss(price, nextPrice, prevPos, -pos)

	if longPL > shortPL {
		return -pos
	} else {
		return pos
	}
}

func (agent *RolloutAgent) profitLoss(price1, price2 float64, pos0, pos1 int) float64 {
	posChg := math.Abs(float64(pos1 - pos0))
	tcost := posChg * agent.TransactionCost

	profitLoss := (price2 - price1) * float64(pos1)

	return profitLoss - tcost
}

func (agent *RolloutAgent) rollout(price float64) float64 {
	for d := 0; d < agent.Depth; d++ {
		prob0 := agent.reverter.Prob0()
		pred := 1
		if rand.Float64() < prob0 {
			pred = 0
		}

		if pred == 1 {
			price *= (1 + agent.Threashold)
		} else {
			price *= (1 - agent.Threashold)
		}
		agent.reverter.Observe(pred)
	}

	for d := 0; d < agent.Depth; d++ {
		agent.reverter.Unobserve()
	}

	return price
}

func run(config Config) error {
	data, err := NewData(config)
	if err != nil {
		return errors.Wrap(err, "")
	}

	wrapper := NewRenkoWrapper(config)
	wrapper.Agent = &NextStep{Leverage: config.Leverage}
	wrapper.Agent = &RolloutAgent{Threashold: config.Threashold, TransactionCost: config.TransactionCost, Leverage: config.Leverage, Depth: 10, NumSimulations: 4096}

	prevCandle, err := data.Read()
	if err != nil {
		return errors.Wrap(err, "")
	}
	for {
		wrapper.Observe(prevCandle)
		candle, err := data.Read()
		if err != nil {
			return errors.Wrap(err, "")
		}
		prevCandle = candle

		if candle.Time.After(time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)) {
			break
		}
	}

	tester := NewTester(config, prevCandle)
	for {
		prev := tester.History[len(tester.History)-1]
		action, rk := wrapper.Act(prevCandle, prev.Balance, prev.Position)
		candle, err := data.Read()
		if err != nil {
			if errors.Cause(err) == io.EOF {
				break
			}
			return errors.Wrap(err, "")
		}
		prevCandle = candle

		tester.Record(action, candle)

		if rk != nil {
			tester.PrintCSV()
		}
	}

	return nil
}

type Config struct {
	Seed            int64
	Data            string
	Threashold      float64
	TransactionCost float64
	Depth           int
	Leverage        float64
	Balance         float64
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
	rand.Seed(config.Seed)
	if err := run(config); err != nil {
		log.Fatalf("%+v", err)
	}
}

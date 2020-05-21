package main

import (
        "encoding/csv"
        "encoding/json"
        "flag"
        "fmt"
	"io"
        "log"
	"math"
        "os"
	"strconv"
	"time"

        "github.com/fumin/ctw"
        "github.com/pkg/errors"
)

var (
        flagConfig = flag.String("c", `{
                "Data": "/Users/mac/Downloads/es_1m.csv",
		"Threashold": 0.002,
                "TransactionCost": 0.05,
                "Depth": 48,
		"Leverage": 1,
		"Balance": 10000
                }`, "configuration")
)

type Data struct{
	Threashold float64
	f *os.File
	r *csv.Reader
	curCandle Candle
}

func NewData(config Config) (*Data, error) {
	data := &Data{}
	data.Threashold = config.Threashold

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

	data.curCandle, err = data.read()
	if err != nil {
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

type Renko struct{
	Time time.Time
	Price float64
	Direction int
}

func (data *Data) Renko() (Renko, error) {
	for {
		cnd, err := data.read()
		if err != nil {
			return Renko{}, errors.Wrap(err, "")
		}
		if cnd.Close / data.curCandle.Close > 1 + data.Threashold {
			data.curCandle = cnd
			return Renko{Time: cnd.Time, Price: cnd.Close, Direction: 1}, nil
		}
		if cnd.Close / data.curCandle.Close < 1 - data.Threashold {
			data.curCandle = cnd
			return Renko{Time: cnd.Time, Price: cnd.Close, Direction: 0}, nil
		}
	}
}

type Candle struct{
	Time time.Time
	Open float64
	High float64
	Low float64
	Close float64
	Volume int64
}

func (data *Data) read() (Candle, error) {
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

type Entry struct{
	Time time.Time
	Price float64
	Position int
	TransactionCost float64
	ProfitLoss float64
	Balance float64
}

type Tester struct{
	TransactionCost float64
	History []Entry

	Trials float64
	Corrects float64
}

func NewTester(config Config, prevRenko Renko) *Tester {
	tester := &Tester{}
	tester.TransactionCost = config.TransactionCost

	entry := Entry{}
	entry.Time = prevRenko.Time
	entry.Price = prevRenko.Price
	entry.Balance = config.Balance
	tester.History = append(tester.History, entry)

	return tester
}

func (tester *Tester) Record(position int, rk Renko) {
	prev := tester.History[len(tester.History)-1]

	posChg := math.Abs(float64(position - prev.Position))
	tcost := posChg * tester.TransactionCost

	profitLoss := (rk.Price - prev.Price) * float64(prev.Position)

	entry := Entry{}
	entry.Time = rk.Time
	entry.Price = rk.Price
	entry.Position = position
	entry.TransactionCost = tcost
	entry.ProfitLoss = profitLoss
	entry.Balance = prev.Balance - tcost + profitLoss
	tester.History = append(tester.History, entry)

	if prev.Position != 0 {
		tester.Trials += 1
		if profitLoss > 0 {
			tester.Corrects += 1
		}
	}

	tester.PrintCSV()
}

func (tester *Tester) PrintCSV() {
	h := tester.History[len(tester.History)-1]
	tStr := h.Time.Format("2006-01-02 15:04")
	fmt.Printf("%s,%.2f,%d,%.2f,%.2f,%.2f\n", tStr, h.Price, h.Position, h.TransactionCost, h.ProfitLoss, h.Balance)
}

func (tester *Tester) Contracts() int {
	contracts := 0
	for i, h := range tester.History[1:] {
		posChg := h.Position - tester.History[i].Position
		if posChg < 0 {
			posChg = -posChg
		}
		contracts += posChg
	}
	return contracts
}

type NextStep struct{
	Leverage float64
	Model *ctw.CTW
}

func (agent *NextStep) Act(price, balance float64, prevPos int) int {
	pos := int(balance / price * agent.Leverage)

	prob0 := agent.Model.Prob0()
	if prob0 > 0.5 {
		pos = -pos
	}

	return pos
}

func run(config Config) error {
	data, err := NewData(config)
	if err != nil {
		return errors.Wrap(err, "")
	}

	context := make([]int, 0, config.Depth)
        for i := 0; i < config.Depth; i++ {
                rk, err := data.Renko()
		if err != nil {
			return errors.Wrap(err, "")
		}
                context = append(context, rk.Direction)
        }
        model := ctw.NewCTW(context)

	// Train.
	var prevRenko Renko
	for {
		rk, err := data.Renko()
		if err != nil {
			return errors.Wrap(err, "")
		}
		model.Observe(rk.Direction)

		if rk.Time.After(time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)) {
			prevRenko = rk
			break
		}
	}

	// Test.
	tester := NewTester(config, prevRenko)
	agent := NextStep{Leverage: config.Leverage, Model: model}
	for {
		prev := tester.History[len(tester.History)-1]
		position := agent.Act(prev.Price, prev.Balance, prev.Position)

                rk, err := data.Renko()
                if err != nil {
                        if errors.Cause(err) == io.EOF {
                                break
                        }
                        return errors.Wrap(err, "")
                }

                model.Observe(rk.Direction)
		tester.Record(position, rk)
        }
	log.Printf("accuracy: %f", tester.Corrects / tester.Trials)
	log.Printf("contracts: %d", tester.Contracts())

	return nil
}

type Config struct {
        Data  string
	Threashold float64
        TransactionCost float64
        Depth int
	Leverage float64
	Balance float64
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

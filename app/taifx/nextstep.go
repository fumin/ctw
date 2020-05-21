package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
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
		"Data": "txf_renko_0001.csv",
		"Depth": 48
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
	Prediction int
	ProfitLoss float64
	Balance    float64
}

type Stat struct {
	Leverage float64
	Items    []StatItem
}

func NewStat(balance, leverage float64) *Stat {
	s := &Stat{}
	s.Leverage = leverage
	s.Items = make([]StatItem, 0, 1024)

	item := StatItem{}
	item.Balance = balance
	s.Items = append(s.Items, item)

	return s
}

func (s *Stat) Record(curBar Bar, prob0 float64, nextBar Bar) {
	prediction := 0
	if prob0 < 0.5 {
		prediction = 1
	}

	prevItem := s.Items[len(s.Items)-1]

	item := StatItem{}
	item.Time = nextBar.Time
	item.Price = nextBar.Price
	item.Prediction = prediction

	profitLoss := nextBar.Price - curBar.Price
	if prediction == 0 {
		profitLoss *= -1
	}
	positionSize := math.Floor(prevItem.Balance / curBar.Price * s.Leverage)
	profitLoss *= positionSize

	item.ProfitLoss = profitLoss
	item.Balance = prevItem.Balance + profitLoss

	s.Items = append(s.Items, item)
}

func (s *Stat) Bankrupt() bool {
	item := s.Items[len(s.Items)-1]
	if item.Balance < 0 {
		return true
	}
	return false
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

	for {
		if trainData.Cursor >= len(trainData.Bar) {
			break
		}
		bar := trainData.Consume()
		model.Observe(bar.Direction)
	}

	testStat := NewStat(20000, 1)
	curBar := trainData.Bar[len(trainData.Bar)-1]
	for {
		prob0 := model.Prob0()
		if testData.Cursor >= len(testData.Bar) {
			break
		}
		nextBar := testData.Consume()

		testStat.Record(curBar, prob0, nextBar)
		if testStat.Bankrupt() {
			break
		}

		model.Observe(nextBar.Direction)
		curBar = nextBar
	}

	fmt.Printf("time,price,prediction,profitloss,balance\n")
	for _, s := range testStat.Items {
		fmt.Printf("%s,%.0f,%d,%.0f,%.0f\n", s.Time.Format("2006-01-02 15:04:05"), s.Price, s.Prediction, s.ProfitLoss, s.Balance)
	}

	return nil
}

type Config struct {
	Data  string
	Depth int
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
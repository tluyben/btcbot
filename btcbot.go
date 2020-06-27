package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shopspring/decimal"
	"github.com/toorop/go-bittrex"
)

var opts struct {
	Prefix   string `short:"p" long:"prefix" description:"A prefix for the environment variables." default:""`
	Database string `short:"d" long:"database" description:"Name of the sqlite3 db to store progress in. Env: {Prefix}DATABASE" required:"false"`
	Key      string `short:"k" long:"key" description:"Bittrex API Key. Env: {Prefix}BITTREX_KEY" required:"false"`
	Secret   string `short:"s" long:"secret" description:"Bittrex API Secret. Env: {Prefix}BITTREX_SECRET" required:"false"`
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func main() {

	_, err := flags.ParseArgs(&opts, os.Args)

	if err != nil {
		log.Fatalf("Could not parse input parameters: %s", err)
	}

	database := os.Getenv(opts.Prefix + "DATABASE")
	key := os.Getenv(opts.Prefix + "BITTREX_KEY")
	secret := os.Getenv(opts.Prefix + "BITTREX_SECRET")

	if opts.Database != "" {
		database = opts.Database
	}
	if opts.Key != "" {
		key = opts.Key
	}
	if opts.Secret != "" {
		secret = opts.Secret
	}

	if database == "" || key == "" || secret == "" {
		log.Fatal("Please specify a database + Bittrex key and secret!")
	}

	dbExists := fileExists(database)

	db, err := sql.Open("sqlite3", database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if !dbExists {
		sqlStmt := `
        create table if not exists btcprice (id integer not null primary key, rate integer, amount integer, buytime datetime);
        CREATE INDEX if not exists btcprice_rate ON btcprice (rate);

        create table if not exists config (id integer not null primary key, usdbalance integer, buyrate integer, amount integer, stagnantwait integer);
		
		insert into config (usdbalance, buyrate, amount, stagnantwait) values (1000000, 9200000, 100000, 24);
		`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return
		}

		fmt.Println("The database has been created with the default values usdbalance = 1000.000, buyrate = 9200.000, amount = 100.000 and stagnantwait = 24.")
		fmt.Println("Press any key to continue with these values; press ctrl-c to change the values in the database yourself. ")
		fmt.Scanln()
	}

	bittrex := bittrex.New(key, secret)
	rows, err := db.Query("select usdbalance, buyrate, amount, stagnantwait as swait from config limit 1")
	if err != nil {
		log.Fatal(err)
	}
	rows.Next()

	var usdbalance int
	var buyrate int
	var amount int
	var swait int

	err = rows.Scan(&usdbalance, &buyrate, &amount, &swait)
	if err != nil {
		log.Fatal(err)
	}

	for {
		//t := time.Now().UTC()

		ticker, err := bittrex.GetTicker("USD-BTC")
		fmt.Println(ticker, err)

		if err != nil {
			fmt.Println(err)

		} else {
			last := int(ticker.Last.Mul(decimal.NewFromInt(1000)).IntPart())

			// check if we need to sell anything:
			stmt, err := db.Prepare("select id, amount, rate from btcprice where rate <= ? order by rate asc limit 1")
			if err != nil {
				log.Fatal(err)
			}
			defer stmt.Close()
			rows, err := stmt.Query(last - amount)
			if err != nil {
				log.Fatal(err)
			}
			// yes
			if rows.Next() {
				var _id int
				var _amount int
				var _rate int

				err = rows.Scan(&_id, &_amount, &_rate)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Printf("Going to sell (Id %d, Amount %d, Rate %d) at $ %d => Profit: %d\n", _id, _amount, _rate, last, last-_rate-int(0.0015*float64(last)))

			}

			// check if we need to buy anything
			if last <= buyrate-amount {
				fmt.Printf("Going to buy at rate %d\n", last)

				buyrate = last
			}

		}
		time.Sleep(time.Second * 5)
	}

}

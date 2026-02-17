package main

import (
	"log"

	"github.com/Krunis/summary-aggregation/packages/aggregator"
	"github.com/Krunis/summary-aggregation/packages/common"
)

func main() {
	port := ":8081"

	aggreg := aggregator.NewAggregatorService(port)

	if err := aggreg.Start(common.GetPostgresConnectionString()); err != nil{
		log.Printf("Error while working: %s", err)
	}
}
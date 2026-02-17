package main

import (
	"log"

	aggreagationserver "github.com/Krunis/summary-aggregation/packages/aggregation_server"
)

func main() {
	port := ":8080"

	aggreagatorAddress := "aggregator:8081"

	as := aggreagationserver.NewAggregationServerService(port)

	if err := as.Start(aggreagatorAddress); err != nil{
		log.Printf("Error while working: %s", err)
	}

}
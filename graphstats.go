// Copyright 2019 Bonsai Software, Inc.  All Rights Reserved.

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/lightningnetwork/lnd/lnrpc"
)

type nodeStats struct {
	PubKey        string
	NumChannels   int
	TotalCapacity int64
	WeightedBase  float64
	WeightedRate  float64
}

func graphStats() {
	file, err := os.Create("graphstats.csv")
	if err != nil {
		fmt.Println("os.Create", "graphstats.csv", "failed:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write the CSV header
	row := []string{"pubkey", "nchannels", "capacity", "base", "rate", "alias"}
	err = writer.Write(row)

	channelGraph, err := gClient.DescribeGraph(gCtx,
		&lnrpc.ChannelGraphRequest{},
	)
	if err != nil {
		fmt.Println("DescribeGraph failed:", err)
		return
	}

	numNodes := int(0)
	numChannels := int(0)
	missingPolicy := int(0)

	for _, node := range channelGraph.Nodes {
		numNodes += 1

		if gCfg.Verbose {
			fmt.Println(node.PubKey)
		}
		nodeInfo, err := gClient.GetNodeInfo(gCtx,
			&lnrpc.NodeInfoRequest{
				PubKey:          node.PubKey,
				IncludeChannels: true,
			},
		)
		if err != nil {
			fmt.Println("GetNodeInfo", node.PubKey, "failed:", err)
			return
		}

		if gCfg.Verbose {
			fmt.Println(nodeInfo.Node.Alias)
		}

		nodeChannels := int(0)
		nodeCapacity := float64(0)
		prodBase := float64(0)
		prodRate := float64(0)
		for _, channelEdge := range nodeInfo.Channels {
			if true {
				// WORKAROUND - Seems the Node1Policy/Node2Policy
				// are twisted when IncludeChannels is used?
				// Instead manually fetch the channel info.
				// SEE: https://github.com/lightningnetwork/lnd/issues/3426
				//
				channelEdge, err = gClient.GetChanInfo(gCtx,
					&lnrpc.ChanInfoRequest{
						ChanId: channelEdge.ChannelId,
					},
				)
				if err != nil {
					fmt.Println("GetChanInfo",
						channelEdge.ChannelId, "failed:", err)
					return
				}
			}

			// Ensure we are match one direction exactly
			isNode1 := node.PubKey == channelEdge.Node1Pub
			isNode2 := node.PubKey == channelEdge.Node2Pub
			if isNode1 != !isNode2 {
				panic("isNode1 != !isNode2")
			}

			// Which policy is in effect here?
			var policy *lnrpc.RoutingPolicy
			if isNode1 {
				policy = channelEdge.Node1Policy
			} else {
				policy = channelEdge.Node2Policy
			}
			// Sometimes there is no policy?
			if policy == nil {
				missingPolicy += 1
				continue
			}

			numChannels += 1
			nodeChannels += 1

			// Aggregate the capacity and weighted base and rate.
			fmt.Println(
				isNode1,
				isNode2,
				channelEdge.ChannelId,
				channelEdge.Capacity,
				(*policy).FeeBaseMsat,
				(*policy).FeeRateMilliMsat,
			)
			nodeCapacity += float64(channelEdge.Capacity)
			prodBase += float64(channelEdge.Capacity) *
				float64((*policy).FeeBaseMsat)
			prodRate += float64(channelEdge.Capacity) *
				float64((*policy).FeeRateMilliMsat)
		}
		if nodeCapacity > 0 {
			weightedBase := (prodBase / nodeCapacity) / 1000 // sat
			weightedRate := (prodRate / nodeCapacity) / 100  // bps
			if gCfg.Verbose {
				fmt.Println("NodeChannels", nodeChannels)
				fmt.Println("NodeCapacity", nodeCapacity)
				fmt.Println("WeightedBase", weightedBase, "sat")
				fmt.Println("WeightedRate", weightedRate, "bps")
			}

			row := []string{
				node.PubKey,                // pubkey
				strconv.Itoa(nodeChannels), // nchannels
				strconv.FormatFloat(nodeCapacity, 'f', -1, 64), // capacity
				strconv.FormatFloat(weightedBase, 'f', -1, 64), // base
				strconv.FormatFloat(weightedRate, 'f', -1, 64), // rate
				nodeInfo.Node.Alias,                            // alias
			}
			_ = writer.Write(row)
		}
		fmt.Println()
	}
	if gCfg.Verbose {
		fmt.Println("NumNodes", numNodes)
		fmt.Println("NumChannels", numChannels)
		fmt.Println("MissingPolicy", missingPolicy)
	}
}
